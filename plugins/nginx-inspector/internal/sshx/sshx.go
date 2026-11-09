// Package sshx maintains cached SSH clients per stored profile and runs
// bounded remote commands. Every operation honors the invocation context and
// caps output so no result can exceed the protocol frame budget.
package sshx

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/rpcerr"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/store"
	plugin "github.com/Syngnat/GoNavi/sdk/go"
	"golang.org/x/crypto/ssh"
)

const (
	// DialTimeout bounds the TCP connect and the SSH handshake wait.
	DialTimeout = 10 * time.Second
	// DefaultExecTimeout bounds one remote command when the caller passes none.
	DefaultExecTimeout = 20 * time.Second
	// MaxOutputBytes caps stdout/stderr of one remote command.
	MaxOutputBytes = 2 << 20
	// keepaliveInterval probes an idle cached connection. A NAT or firewall
	// that drops idle sessions otherwise surfaces as a mid-command EOF.
	keepaliveInterval = 30 * time.Second
	keepaliveTimeout  = 5 * time.Second
	keepaliveFailures = 3
)

// Probe reasons reported by Pool.Probe besides success.
const (
	// ProbeUnreachable means the remote host refused or timed out the TCP
	// connection to the member; this is a normal "down" answer.
	ProbeUnreachable = "unreachable"
	// ProbeForwardingDisabled means sshd refused the direct-tcpip channel,
	// which is what AllowTcpForwarding no looks like from the client side.
	ProbeForwardingDisabled = "tcp-forwarding-disabled"
)

// ExecResult carries one bounded remote command outcome.
type ExecResult struct {
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   int    `json:"exitCode"`
	Truncated  bool   `json:"truncated"`
	DurationMs int64  `json:"durationMs"`
}

// ReadResult is one remote file read.
type ReadResult struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	SizeBytes int64  `json:"sizeBytes"`
	Truncated bool   `json:"truncated"`
}

// HostKeyError reports a mismatch between the stored and the presented
// server fingerprint. Details are safe: fingerprints are public material.
type HostKeyError struct {
	Expected string
	Actual   string
}

func (e *HostKeyError) Error() string { return "server host key changed" }

// Pool caches one SSH client per profile id.
type Pool struct {
	mu      sync.Mutex
	clients map[string]*ssh.Client
	mems    map[string]string // in-memory fingerprints when the store is unavailable
	st      *store.Store
}

// NewPool returns a pool persisting first-seen fingerprints into st (may be nil).
func NewPool(st *store.Store) *Pool {
	return &Pool{clients: map[string]*ssh.Client{}, mems: map[string]string{}, st: st}
}

// Close terminates every cached client.
func (p *Pool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for id, c := range p.clients {
		_ = c.Close()
		delete(p.clients, id)
	}
}

// Invalidate drops the cached client for one profile.
func (p *Pool) Invalidate(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if c, ok := p.clients[id]; ok {
		_ = c.Close()
		delete(p.clients, id)
	}
}

func (p *Pool) cached(id string) (*ssh.Client, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	c, ok := p.clients[id]
	return c, ok
}

func (p *Pool) remember(id string, c *ssh.Client) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if old, ok := p.clients[id]; ok && old != c {
		_ = old.Close()
	}
	p.clients[id] = c
}

func (p *Pool) recordFingerprint(id, fingerprint string) {
	if p.st != nil {
		_ = p.st.SetFingerprint(id, fingerprint)
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.mems[id] == "" {
		p.mems[id] = fingerprint
	}
}

func (p *Pool) hostKeyCallback(prof store.Profile) ssh.HostKeyCallback {
	return func(_ string, _ net.Addr, key ssh.PublicKey) error {
		fingerprint := ssh.FingerprintSHA256(key)
		expected := prof.Fingerprint
		if expected == "" && p.st == nil {
			p.mu.Lock()
			expected = p.mems[prof.ID]
			p.mu.Unlock()
		}
		if expected == "" {
			p.recordFingerprint(prof.ID, fingerprint)
			return nil
		}
		if expected != fingerprint {
			return &HostKeyError{Expected: expected, Actual: fingerprint}
		}
		return nil
	}
}

func authMethods(prof store.Profile) ([]ssh.AuthMethod, *plugin.Error) {
	switch prof.AuthType {
	case store.AuthPassword:
		return []ssh.AuthMethod{ssh.Password(prof.Password)}, nil
	case store.AuthKey:
		key, err := os.ReadFile(store.NormalizeKeyPath(prof.KeyPath))
		if err != nil {
			return nil, rpcerr.E(rpcerr.CodeSSHAuthFailed, "read private key: "+err.Error())
		}
		var signer ssh.Signer
		if prof.KeyPassphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(prof.KeyPassphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(key)
		}
		if err != nil {
			var missing *ssh.PassphraseMissingError
			if errors.As(err, &missing) {
				return nil, rpcerr.E(rpcerr.CodeSSHAuthFailed, "private key requires a passphrase")
			}
			return nil, rpcerr.E(rpcerr.CodeSSHAuthFailed, "parse private key: "+err.Error())
		}
		return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
	default:
		return nil, rpcerr.E(rpcerr.CodeInvalidArgs, "unsupported auth type")
	}
}

// Client returns a live SSH client for the profile, dialing when needed.
func (p *Pool) Client(ctx context.Context, prof store.Profile) (*ssh.Client, *plugin.Error) {
	if c, ok := p.cached(prof.ID); ok {
		return c, nil
	}
	client, perr := p.dial(ctx, prof)
	if perr != nil {
		return nil, perr
	}
	p.remember(prof.ID, client)
	p.startKeepalive(prof.ID, client)
	return client, nil
}

// startKeepalive keeps a cached connection warm and evicts it after three
// failed keepalives, so the next command dials afresh instead of reusing a
// half-open socket. The goroutine exits when the client is closed.
func (p *Pool) startKeepalive(id string, client *ssh.Client) {
	closed := make(chan struct{})
	go func() {
		_ = client.Wait()
		close(closed)
	}()
	go func() {
		ticker := time.NewTicker(keepaliveInterval)
		defer ticker.Stop()
		failures := 0
		for {
			select {
			case <-closed:
				return
			case <-ticker.C:
				if p.keepaliveOnce(client) {
					failures = 0
					continue
				}
				failures++
				if failures >= keepaliveFailures {
					p.Invalidate(id)
					return
				}
			}
		}
	}()
}

// keepaliveOnce sends one keepalive request with a bounded wait. The request
// goroutine can stay blocked until the connection closes; callers only reach
// that state after the connection is already considered dead.
func (p *Pool) keepaliveOnce(client *ssh.Client) bool {
	type reply struct {
		ok  bool
		err error
	}
	done := make(chan reply, 1)
	go func() {
		ok, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
		done <- reply{ok: ok, err: err}
	}()
	select {
	case res := <-done:
		// Any reply means the transport is alive. Some servers answer
		// keepalive@openssh.com with REQUEST_FAILURE, which is not a dead
		// connection and must not evict a healthy client.
		return res.err == nil
	case <-time.After(keepaliveTimeout):
		return false
	}
}

func (p *Pool) dial(ctx context.Context, prof store.Profile) (*ssh.Client, *plugin.Error) {
	methods, perr := authMethods(prof)
	if perr != nil {
		return nil, perr
	}
	addr := net.JoinHostPort(prof.Host, strconv.Itoa(prof.Port))
	config := &ssh.ClientConfig{
		User:            prof.Username,
		Auth:            methods,
		HostKeyCallback: p.hostKeyCallback(prof),
		Timeout:         DialTimeout,
	}
	dialer := &net.Dialer{Timeout: DialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		if ctx.Err() != nil {
			return nil, rpcerr.E(rpcerr.CodeSSHTimeout, "SSH dial canceled")
		}
		return nil, rpcerr.FromCode(err)
	}
	type handshake struct {
		client *ssh.Client
		err    error
	}
	done := make(chan handshake, 1)
	go func() {
		conn, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
		if err != nil {
			done <- handshake{err: err}
			return
		}
		done <- handshake{client: ssh.NewClient(conn, chans, reqs)}
	}()
	handshakeTimer := time.NewTimer(DialTimeout)
	defer handshakeTimer.Stop()
	select {
	case <-ctx.Done():
		_ = conn.Close()
		go func() {
			if res := <-done; res.client != nil {
				_ = res.client.Close()
			}
		}()
		return nil, rpcerr.E(rpcerr.CodeSSHTimeout, "SSH handshake canceled")
	case <-handshakeTimer.C:
		_ = conn.Close()
		go func() {
			if res := <-done; res.client != nil {
				_ = res.client.Close()
			}
		}()
		return nil, rpcerr.E(rpcerr.CodeSSHTimeout, "SSH handshake timed out")
	case res := <-done:
		if res.err != nil {
			_ = conn.Close()
			var hostKeyErr *HostKeyError
			if errors.As(res.err, &hostKeyErr) {
				return nil, rpcerr.E(rpcerr.CodeHostKeyChanged, "server host key changed; verify and update the connection",
					map[string]string{"expected": hostKeyErr.Expected, "actual": hostKeyErr.Actual})
			}
			return nil, rpcerr.FromCode(res.err)
		}
		return res.client, nil
	}
}

type limitedBuffer struct {
	limit     int
	buf       strings.Builder
	truncated bool
}

func (b *limitedBuffer) Write(data []byte) (int, error) {
	room := b.limit - b.buf.Len()
	if room > 0 {
		if len(data) > room {
			b.buf.Write(data[:room])
			b.truncated = true
		} else {
			b.buf.Write(data)
		}
	} else {
		b.truncated = true
	}
	return len(data), nil
}

func retryable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, marker := range []string{
		"EOF", "closed network connection", "connection reset", "connection lost",
		"broken pipe", "i/o timeout", "connection refused", "handshake failed",
		"no route to host", "network is unreachable",
	} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

// Run executes one remote command with bounded output. A nonzero remote exit
// is returned as result, not as an error. Transport errors on a cached client
// are retried once on a fresh connection, so Run is only for idempotent
// commands (reads, probes, status queries).
func (p *Pool) Run(ctx context.Context, prof store.Profile, command string, timeout time.Duration, limit int64) (ExecResult, *plugin.Error) {
	return p.run(ctx, prof, command, timeout, limit, true)
}

// RunOnce executes one remote command without the reconnect-and-retry step.
// Commands with side effects must use it: after a timeout or a dropped
// connection the command may already have reached the remote host, and
// replaying it would repeat the effect.
func (p *Pool) RunOnce(ctx context.Context, prof store.Profile, command string, timeout time.Duration, limit int64) (ExecResult, *plugin.Error) {
	return p.run(ctx, prof, command, timeout, limit, false)
}

func (p *Pool) run(ctx context.Context, prof store.Profile, command string, timeout time.Duration, limit int64, retry bool) (ExecResult, *plugin.Error) {
	if timeout <= 0 {
		timeout = DefaultExecTimeout
	}
	if limit <= 0 || limit > MaxOutputBytes {
		limit = MaxOutputBytes
	}
	client, ok := p.cached(prof.ID)
	if ok {
		result, err := runOnce(ctx, client, command, timeout, limit)
		if err == nil {
			return result, nil
		}
		if !retry || !retryable(err) {
			return ExecResult{}, rpcerr.FromCode(err)
		}
		p.Invalidate(prof.ID)
	}
	client, perr := p.Client(ctx, prof)
	if perr != nil {
		return ExecResult{}, perr
	}
	result, err := runOnce(ctx, client, command, timeout, limit)
	if err != nil {
		return ExecResult{}, rpcerr.FromCode(err)
	}
	return result, nil
}

func runOnce(ctx context.Context, client *ssh.Client, command string, timeout time.Duration, limit int64) (ExecResult, error) {
	start := time.Now()
	session, err := client.NewSession()
	if err != nil {
		return ExecResult{}, err
	}
	defer session.Close()

	stdout, stderr := &limitedBuffer{limit: int(limit)}, &limitedBuffer{limit: 64 << 10}
	session.Stdout, session.Stderr = stdout, stderr
	done := make(chan error, 1)
	go func() { done <- session.Run(command) }()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	var waitErr error
	select {
	case <-ctx.Done():
		_ = session.Close()
		awaitSession(done)
		return ExecResult{}, ctx.Err()
	case <-timer.C:
		_ = session.Close()
		awaitSession(done)
		return ExecResult{}, fmt.Errorf("remote command timed out after %s", timeout)
	case waitErr = <-done:
	}
	if waitErr != nil {
		var exitErr *ssh.ExitError
		if errors.As(waitErr, &exitErr) {
			return ExecResult{
				Stdout: stdout.buf.String(), Stderr: stderr.buf.String(),
				ExitCode: exitErr.ExitStatus(), Truncated: stdout.truncated,
				DurationMs: time.Since(start).Milliseconds(),
			}, nil
		}
		return ExecResult{}, waitErr
	}
	return ExecResult{
		Stdout: stdout.buf.String(), Stderr: stderr.buf.String(),
		ExitCode: 0, Truncated: stdout.truncated,
		DurationMs: time.Since(start).Milliseconds(),
	}, nil
}

// awaitSession bounds the wait for ssh.Session.Run to unwind after Close.
// A black-holed TCP connection can keep the channel bookkeeping blocked, and
// a diagnostic handler must never hang on it forever.
func awaitSession(done <-chan error) {
	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}
}

// Output runs a command and requires a zero exit status.
func (p *Pool) Output(ctx context.Context, prof store.Profile, command string, timeout time.Duration) (string, *plugin.Error) {
	result, perr := p.Run(ctx, prof, command, timeout, 0)
	if perr != nil {
		return "", perr
	}
	if result.ExitCode != 0 {
		detail := strings.TrimSpace(result.Stderr)
		if detail == "" {
			detail = strings.TrimSpace(result.Stdout)
		}
		return "", rpcerr.E(rpcerr.CodeExecFailed, "remote command failed with exit code "+strconv.Itoa(result.ExitCode), map[string]string{
			"command":  command,
			"exitCode": strconv.Itoa(result.ExitCode),
			"stderr":   truncateText(detail, 512),
		})
	}
	return result.Stdout, nil
}

// ReadFile reads a remote file through cat with a byte cap. The path must be
// absolute and is single-quoted before it reaches the remote shell.
func (p *Pool) ReadFile(ctx context.Context, prof store.Profile, path string, maxBytes int64) (ReadResult, *plugin.Error) {
	if !strings.HasPrefix(path, "/") || len(path) > 4096 || strings.ContainsAny(path, "\x00\n\r") {
		return ReadResult{}, rpcerr.E(rpcerr.CodeInvalidArgs, "remote path must be absolute")
	}
	quoted := ShellQuote(path)
	sizeRun, perr := p.Run(ctx, prof, "wc -c < "+quoted, 10*time.Second, 4096)
	if perr != nil {
		return ReadResult{}, perr
	}
	if sizeRun.ExitCode != 0 {
		return ReadResult{}, rpcerr.E(rpcerr.CodeFileUnreadable, "remote file is not readable: "+path,
			map[string]string{"stderr": truncateText(strings.TrimSpace(sizeRun.Stderr), 256)})
	}
	size, _ := strconv.ParseInt(strings.TrimSpace(sizeRun.Stdout), 10, 64)
	result, perr := p.Run(ctx, prof, "cat "+quoted, DefaultExecTimeout, maxBytes+1)
	if perr != nil {
		return ReadResult{}, perr
	}
	if result.ExitCode != 0 {
		return ReadResult{}, rpcerr.E(rpcerr.CodeFileUnreadable, "remote file is not readable: "+path,
			map[string]string{"stderr": truncateText(strings.TrimSpace(result.Stderr), 256)})
	}
	content := result.Stdout
	truncated := result.Truncated || (maxBytes > 0 && int64(len(content)) > maxBytes)
	if truncated && int64(len(content)) > maxBytes {
		content = content[:maxBytes]
	}
	return ReadResult{Path: path, Content: content, SizeBytes: size, Truncated: truncated}, nil
}

// Probe dials one TCP address from the remote host. It returns the round
// trip in milliseconds on success, -1 plus a reason when the member does not
// answer, and a plugin.Error only when the SSH transport itself fails.
//
// A late connection is always closed: when the deadline wins the race the
// dial goroutine may still deliver a live conn afterwards, and abandoning it
// would leak a forwarded socket on the remote sshd.
func (p *Pool) Probe(ctx context.Context, prof store.Profile, host string, port int, timeout time.Duration) (int64, string, *plugin.Error) {
	client, perr := p.Client(ctx, prof)
	if perr != nil {
		return -1, "", perr
	}
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	type outcome struct {
		conn net.Conn
		err  error
	}
	done := make(chan outcome, 1)
	closeLate := func() {
		go func() {
			if res := <-done; res.conn != nil {
				_ = res.conn.Close()
			}
		}()
	}
	start := time.Now()
	go func() {
		conn, err := client.Dial("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
		done <- outcome{conn: conn, err: err}
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		closeLate()
		return -1, "", rpcerr.E(rpcerr.CodeSSHTimeout, "probe canceled")
	case <-timer.C:
		closeLate()
		return -1, ProbeUnreachable, nil
	case res := <-done:
		if res.conn != nil {
			_ = res.conn.Close()
		}
		if res.err != nil {
			return -1, probeFailureReason(res.err), nil
		}
		return time.Since(start).Milliseconds(), "", nil
	}
}

// probeFailureReason distinguishes a forwarding policy denial (which makes
// every member "skipped" rather than "down") from an ordinary refusal.
func probeFailureReason(err error) string {
	var openErr *ssh.OpenChannelError
	if errors.As(err, &openErr) && openErr.Reason == ssh.Prohibited {
		return ProbeForwardingDisabled
	}
	return ProbeUnreachable
}

// ShellQuote single-quotes a value for safe interpolation into a remote shell command.
func ShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func truncateText(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// ParseCertificatePEM extracts the first X.509 certificate from PEM content.
// A certificate file may start with a PRIVATE KEY or DH PARAMETERS block, so
// every block is visited until a CERTIFICATE is found.
func ParseCertificatePEM(content []byte) (*x509.Certificate, error) {
	rest := content
	for {
		block, remaining := pem.Decode(rest)
		if block == nil {
			return nil, errors.New("no CERTIFICATE block found")
		}
		rest = remaining
		if block.Type != "CERTIFICATE" {
			if len(rest) == 0 {
				return nil, errors.New("no CERTIFICATE block found")
			}
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		return cert, nil
	}
}
