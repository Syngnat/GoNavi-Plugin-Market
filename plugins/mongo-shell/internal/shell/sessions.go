package shell

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"
)

const (
	maxSessions        = 32
	idleSessionTimeout = 30 * time.Minute
)

// Sessions routes protocol commands to independent clients using opaque bearer
// IDs. The single-client Shell API remains available to existing consumers.
type Sessions struct {
	mu       sync.Mutex
	sessions map[string]*managedSession
	closed   bool
	factory  clientFactory
	limit    int
	now      func() time.Time
	// Logger receives only the same redacted diagnostics as Shell.Logger.
	Logger func(context.Context, string, string)
}

type managedSession struct {
	shell    *Shell
	closed   bool
	calls    map[uint64]context.CancelFunc
	nextCall uint64
	lastUsed time.Time
}

type sessionArgs struct {
	SessionID string `json:"sessionId"`
}

type sessionConnectData struct {
	connectData
	SessionID string `json:"sessionId"`
}

// NewSessions creates a command router capped at 32 live or pending clients.
func NewSessions() *Sessions {
	return &Sessions{sessions: make(map[string]*managedSession), factory: NewMongoClient, limit: maxSessions, now: time.Now}
}

// Connect opens a new session without replacing any other caller's client.
func (m *Sessions) Connect(ctx context.Context, raw json.RawMessage) (any, error) {
	var args connectArgs
	if err := decodeArguments(raw, &args); err != nil {
		return failure(err), nil
	}
	m.reapIdle(ctx)
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return failure(wrapCodedError(codeInternal, "cannot create a shell session", err)), nil
	}
	id := hex.EncodeToString(token[:])
	s := New()
	s.factory, s.Logger = m.factory, m.Logger
	entry := &managedSession{shell: s, calls: make(map[uint64]context.CancelFunc), lastUsed: m.now()}
	callCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	m.mu.Lock()
	if m.closed || len(m.sessions) >= m.limit {
		m.mu.Unlock()
		return failure(newCodedError("session_limit", "the shell is closed or the session limit has been reached")), nil
	}
	entry.calls[0] = cancel
	m.sessions[id] = entry
	m.mu.Unlock()
	data, err := s.connect(callCtx, args)
	m.mu.Lock()
	delete(entry.calls, 0)
	entry.lastUsed = m.now()
	closed := entry.closed || m.closed || callCtx.Err() != nil
	if err != nil || closed {
		entry.closed = true
	}
	m.mu.Unlock()
	if err != nil || closed {
		s.Disconnect(context.WithoutCancel(ctx), nil)
		m.mu.Lock()
		delete(m.sessions, id)
		m.mu.Unlock()
		if err == nil {
			err = newCodedError(codeNotConnected, "the shell session was closed while connecting")
		}
		return failure(err), nil
	}
	return success(sessionConnectData{connectData: data, SessionID: id}), nil
}

func (m *Sessions) acquire(ctx context.Context, raw json.RawMessage) (*managedSession, context.Context, func(), error) {
	var args sessionArgs
	if err := decodeArguments(raw, &args); err != nil {
		return nil, nil, nil, err
	}
	m.mu.Lock()
	entry := m.sessions[args.SessionID]
	if args.SessionID == "" || entry == nil || entry.closed || m.closed {
		m.mu.Unlock()
		return nil, nil, nil, newCodedError(codeNotConnected, "a valid sessionId from connect is required")
	}
	callCtx, cancel := context.WithCancel(ctx)
	entry.nextCall++
	token := entry.nextCall
	entry.calls[token] = cancel
	m.mu.Unlock()
	release := func() {
		cancel()
		m.mu.Lock()
		delete(entry.calls, token)
		entry.lastUsed = m.now()
		m.mu.Unlock()
	}
	return entry, callCtx, release, nil
}

// RunCommand executes only on the explicitly named session.
func (m *Sessions) RunCommand(ctx context.Context, raw json.RawMessage) (any, error) {
	entry, callCtx, release, err := m.acquire(ctx, raw)
	if err != nil {
		return failure(err), nil
	}
	defer release()
	return entry.shell.RunCommand(callCtx, raw)
}

// Ping verifies only the explicitly named session.
func (m *Sessions) Ping(ctx context.Context, raw json.RawMessage) (any, error) {
	entry, callCtx, release, err := m.acquire(ctx, raw)
	if err != nil {
		return failure(err), nil
	}
	defer release()
	return entry.shell.Ping(callCtx, raw)
}

// Disconnect invalidates one ID, cancels its work, and releases its driver.
func (m *Sessions) Disconnect(ctx context.Context, raw json.RawMessage) (any, error) {
	var args sessionArgs
	if err := decodeArguments(raw, &args); err != nil {
		return failure(err), nil
	}
	m.mu.Lock()
	entry := m.sessions[args.SessionID]
	if args.SessionID == "" || entry == nil || entry.closed || m.closed {
		m.mu.Unlock()
		return failure(newCodedError(codeNotConnected, "a valid sessionId from connect is required")), nil
	}
	entry.closed = true
	for _, cancel := range entry.calls {
		cancel()
	}
	m.mu.Unlock()
	value, err := entry.shell.Disconnect(ctx, nil)
	m.mu.Lock()
	delete(m.sessions, args.SessionID)
	m.mu.Unlock()
	return value, err
}

// Abort invalidates all sessions and cancels in-flight work without waiting for
// network IO. Close must follow after the SDK has stopped dispatching handlers.
func (m *Sessions) Abort() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	for _, entry := range m.sessions {
		entry.closed = true
		for _, cancel := range entry.calls {
			cancel()
		}
	}
}

// Close performs bounded driver cleanup after Abort; it starts no goroutines.
func (m *Sessions) Close(ctx context.Context) {
	m.Abort()
	m.mu.Lock()
	entries := m.sessions
	m.sessions = make(map[string]*managedSession)
	m.mu.Unlock()
	for _, entry := range entries {
		entry.shell.Disconnect(ctx, nil)
	}
}

// Reclaim at most one abandoned idle client per connect. There is no sweeper
// goroutine and cleanup costs at most one driver's disconnect budget. Active
// requests, including pending connects, are never expired underneath callers.
func (m *Sessions) reapIdle(ctx context.Context) {
	m.mu.Lock()
	var oldest *managedSession
	var id string
	now := m.now()
	if !m.closed {
		for key, entry := range m.sessions {
			if entry.closed || len(entry.calls) != 0 || now.Sub(entry.lastUsed) < idleSessionTimeout {
				continue
			}
			if oldest == nil || entry.lastUsed.Before(oldest.lastUsed) {
				oldest, id = entry, key
			}
		}
	}
	if oldest != nil {
		oldest.closed = true
	}
	m.mu.Unlock()
	if oldest == nil {
		return
	}
	oldest.shell.Disconnect(ctx, nil)
	m.mu.Lock()
	delete(m.sessions, id)
	m.mu.Unlock()
}
