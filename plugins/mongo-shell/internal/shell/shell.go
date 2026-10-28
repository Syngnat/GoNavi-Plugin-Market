package shell

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultCommandTimeout = 30 * time.Second
	disconnectTimeout     = 5 * time.Second
	defaultDatabase       = "test"
)

// Shell owns the one active MongoDB client of a plugin window process. The
// window bridge can call handlers concurrently, so state changes take the write
// lock while scripts run under the read lock.
type Shell struct {
	opMu           sync.RWMutex
	client         Client
	session        sessionInfo
	factory        clientFactory
	commandTimeout time.Duration
	// Logger is injected by main so the package stays independent of the SDK.
	// It must never receive credentials.
	Logger func(ctx context.Context, level, message string)
}

type sessionInfo struct {
	connectionID    string
	connectionName  string
	environmentType string
	host            string
	database        string
	collection      string
	readOnly        bool
}

// New creates a shell with the real MongoDB driver factory and the documented
// thirty-second command timeout.
func New() *Shell {
	return &Shell{
		factory:        NewMongoClient,
		commandTimeout: defaultCommandTimeout,
	}
}

type connectArgs struct {
	Config          *ConnectionConfig `json:"config"`
	ReadOnly        bool              `json:"readOnly,omitempty"`
	Database        string            `json:"database,omitempty"`
	Collection      string            `json:"collection,omitempty"`
	ConnectionID    string            `json:"connectionId,omitempty"`
	ConnectionName  string            `json:"connectionName,omitempty"`
	EnvironmentType string            `json:"environmentType,omitempty"`
}

type connectData struct {
	ConnectionID    string         `json:"connectionId,omitempty"`
	ConnectionName  string         `json:"connectionName"`
	EnvironmentType string         `json:"environmentType,omitempty"`
	Host            string         `json:"host"`
	Database        string         `json:"database"`
	Collection      string         `json:"collection,omitempty"`
	ReadOnly        bool           `json:"readOnly"`
	Databases       []DatabaseInfo `json:"databases"`
}

type pingData struct {
	LatencyMs int64  `json:"latencyMs"`
	Database  string `json:"database"`
}

type disconnectData struct {
	Disconnected bool `json:"disconnected"`
}

// Connect opens the saved connection, verifies it with a bounded ping and
// returns the database list. Credentials are used once and never retained.
func (s *Shell) Connect(ctx context.Context, raw json.RawMessage) (any, error) {
	var args connectArgs
	if err := decodeArguments(raw, &args); err != nil {
		return failure(err), nil
	}
	data, err := s.connect(ctx, args)
	if err != nil {
		return failure(err), nil
	}
	return success(data), nil
}

func (s *Shell) connect(ctx context.Context, args connectArgs) (connectData, error) {
	if args.Config == nil {
		return connectData{}, newCodedError(codeInvalidArgument, "the launch context did not include the MongoDB connection configuration")
	}
	config := *args.Config
	if err := config.validate(); err != nil {
		return connectData{}, err
	}
	connectCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	client, err := s.factory(config)
	if err != nil {
		return connectData{}, err
	}
	if err := client.Ping(connectCtx); err != nil {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(connectCtx), disconnectTimeout)
		_ = client.Disconnect(cleanupCtx)
		cleanupCancel()
		return connectData{}, wrapCodedError(codeConnectionFailed, "cannot reach the MongoDB server", err)
	}
	databases, err := client.ListDatabases(connectCtx)
	if err != nil {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(connectCtx), disconnectTimeout)
		_ = client.Disconnect(cleanupCtx)
		cleanupCancel()
		return connectData{}, wrapCodedError(codeConnectionFailed, "cannot list databases: the MongoDB server rejected the request", err)
	}
	database := strings.TrimSpace(args.Database)
	if database == "" {
		database = strings.TrimSpace(config.Database)
	}
	if database == "" {
		database = defaultDatabase
	}
	info := sessionInfo{
		connectionID:    strings.TrimSpace(args.ConnectionID),
		connectionName:  strings.TrimSpace(args.ConnectionName),
		environmentType: strings.TrimSpace(args.EnvironmentType),
		host:            displayHost(config),
		database:        database,
		collection:      strings.TrimSpace(args.Collection),
		readOnly:        args.ReadOnly || config.ForcedReadOnly(),
	}
	s.opMu.Lock()
	previous := s.client
	s.client = client
	s.session = info
	s.opMu.Unlock()
	if previous != nil {
		disconnectCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), disconnectTimeout)
		_ = previous.Disconnect(disconnectCtx)
		stop()
	}
	s.log(ctx, "info", "connected to "+info.host+" (readOnly="+boolText(info.readOnly)+")")
	return connectData{
		ConnectionID:    info.connectionID,
		ConnectionName:  info.connectionName,
		EnvironmentType: info.environmentType,
		Host:            info.host,
		Database:        info.database,
		Collection:      info.collection,
		ReadOnly:        info.readOnly,
		Databases:       databases,
	}, nil
}

// Disconnect closes the active client. It is safe to call when not connected.
func (s *Shell) Disconnect(ctx context.Context, _ json.RawMessage) (any, error) {
	s.opMu.Lock()
	client := s.client
	s.client = nil
	s.session = sessionInfo{}
	s.opMu.Unlock()
	if client != nil {
		disconnectCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), disconnectTimeout)
		_ = client.Disconnect(disconnectCtx)
		cancel()
	}
	return success(disconnectData{Disconnected: true}), nil
}

// Ping verifies the active client without running any script.
func (s *Shell) Ping(ctx context.Context, _ json.RawMessage) (any, error) {
	s.opMu.RLock()
	client := s.client
	session := s.session
	s.opMu.RUnlock()
	if client == nil {
		return failure(newCodedError(codeNotConnected, "not connected: open a MongoDB connection first")), nil
	}
	pingCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	started := time.Now()
	if err := client.Ping(pingCtx); err != nil {
		return failure(wrapCodedError(codeConnectionFailed, "the MongoDB ping failed", err)), nil
	}
	return success(pingData{LatencyMs: time.Since(started).Milliseconds(), Database: session.database}), nil
}

// RunCommand executes a JavaScript shell script and returns its output, the
// last expression and the most recent result table.
func (s *Shell) RunCommand(ctx context.Context, raw json.RawMessage) (any, error) {
	var args runArgs
	if err := decodeArguments(raw, &args); err != nil {
		return failure(err), nil
	}
	if strings.TrimSpace(args.Script) == "" {
		return failure(newCodedError(codeInvalidArgument, "the script is empty")), nil
	}
	data, err := s.RunScript(ctx, args)
	if err != nil {
		return failure(err), nil
	}
	if data.Output == nil {
		data.Output = []string{}
	}
	return success(data), nil
}

// Abort releases state without network calls; the SDK calls it on shutdown and
// requires a nonblocking implementation.
func (s *Shell) Abort() {
	s.opMu.Lock()
	s.client = nil
	s.session = sessionInfo{}
	s.opMu.Unlock()
}

func (s *Shell) log(ctx context.Context, level, message string) {
	if s.Logger == nil || strings.TrimSpace(message) == "" {
		return
	}
	if len([]rune(message)) > 512 {
		message = truncateRunes(message, 512)
	}
	s.Logger(ctx, level, message)
}

func decodeArguments(raw json.RawMessage, target any) error {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return newCodedError(codeInvalidArgument, "invalid command arguments: "+err.Error())
	}
	return nil
}

func displayHost(config ConnectionConfig) string {
	if raw := strings.TrimSpace(config.URI); raw != "" {
		if parsed, err := url.Parse(raw); err == nil && parsed.Host != "" {
			return parsed.Host
		}
		return "mongodb"
	}
	if len(config.Hosts) > 0 {
		return strings.Join(config.Hosts, ",")
	}
	host := strings.TrimSpace(config.Host)
	if host == "" {
		host = "localhost"
	}
	if config.Port > 0 {
		return host + ":" + strconv.Itoa(config.Port)
	}
	return host
}

func boolText(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
