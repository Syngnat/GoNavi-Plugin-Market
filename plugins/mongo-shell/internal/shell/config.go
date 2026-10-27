package shell

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultMongoPort = 27017
	// connectTimeout bounds client creation plus the first ping/listDatabases
	// round trip. It is deliberately independent from the script timeout.
	connectTimeout = 10 * time.Second
)

// Protection mirrors the host connection protection flags that decide whether
// a connection is forced read-only.
type Protection struct {
	RestrictDataEdit        bool `json:"restrictDataEdit,omitempty"`
	RestrictStructureEdit   bool `json:"restrictStructureEdit,omitempty"`
	RestrictScriptExecution bool `json:"restrictScriptExecution,omitempty"`
	RestrictDataImport      bool `json:"restrictDataImport,omitempty"`
}

// ConnectionConfig is the subset of the host saved-connection configuration the
// shell needs. Credentials arrive only because the manifest declares
// host.connections.credentials; the shell never persists or returns them.
type ConnectionConfig struct {
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`
	Database string `json:"database,omitempty"`
	ReadOnly bool   `json:"readOnly,omitempty"`

	URI            string   `json:"uri,omitempty"`
	Hosts          []string `json:"hosts,omitempty"`
	Topology       string   `json:"topology,omitempty"`
	ReplicaSet     string   `json:"replicaSet,omitempty"`
	AuthSource     string   `json:"authSource,omitempty"`
	ReadPreference string   `json:"readPreference,omitempty"`

	MongoSRV           bool   `json:"mongoSrv,omitempty"`
	MongoAuthMechanism string `json:"mongoAuthMechanism,omitempty"`
	MongoReplicaUser   string `json:"mongoReplicaUser,omitempty"`

	ConnectionParams string `json:"connectionParams,omitempty"`
	Timeout          int    `json:"timeout,omitempty"`

	UseSSL      bool   `json:"useSSL,omitempty"`
	SSLMode     string `json:"sslMode,omitempty"`
	SSLCAPath   string `json:"sslCAPath,omitempty"`
	SSLCertPath string `json:"sslCertPath,omitempty"`
	SSLKeyPath  string `json:"sslKeyPath,omitempty"`

	UseSSH        bool `json:"useSSH,omitempty"`
	UseProxy      bool `json:"useProxy,omitempty"`
	UseHTTPTunnel bool `json:"useHttpTunnel,omitempty"`

	Protection Protection `json:"protection,omitempty"`
}

// ForcedReadOnly mirrors the host decision for a forced read-only saved
// connection: the launch context flag wins, and legacy/protection flags are ORed
// so a stale UI cannot silently enable writes.
func (c ConnectionConfig) ForcedReadOnly() bool {
	p := c.Protection
	protected := p.RestrictDataEdit && p.RestrictStructureEdit &&
		p.RestrictScriptExecution && p.RestrictDataImport
	return c.ReadOnly || protected
}

func (c ConnectionConfig) validate() error {
	if strings.TrimSpace(c.Type) != "" && !strings.EqualFold(strings.TrimSpace(c.Type), "mongodb") {
		return codedErrorf(codeUnsupportedConnection, "MongoShell only supports mongodb connections, got %q", c.Type)
	}
	if c.UseSSH {
		return newCodedError(codeUnsupportedTransport, "SSH tunnels are not supported by MongoShell v1; use a direct MongoDB connection")
	}
	if c.UseProxy {
		return newCodedError(codeUnsupportedTransport, "HTTP proxies are not supported by MongoShell v1; use a direct MongoDB connection")
	}
	if c.UseHTTPTunnel {
		return newCodedError(codeUnsupportedTransport, "HTTP tunnels are not supported by MongoShell v1; use a direct MongoDB connection")
	}
	return nil
}

// MongoURI returns the connection string handed to the driver. An explicit URI
// is authoritative; otherwise the URI is built from the saved connection.
func (c ConnectionConfig) MongoURI() (string, error) {
	if err := c.validate(); err != nil {
		return "", err
	}
	if raw := strings.TrimSpace(c.URI); raw != "" {
		lower := strings.ToLower(raw)
		if !strings.HasPrefix(lower, "mongodb://") && !strings.HasPrefix(lower, "mongodb+srv://") {
			return "", newCodedError(codeInvalidArgument, "the MongoDB URI must start with mongodb:// or mongodb+srv://")
		}
		return raw, nil
	}
	return c.buildURI()
}

func (c ConnectionConfig) buildURI() (string, error) {
	scheme := "mongodb"
	if c.MongoSRV {
		scheme = "mongodb+srv"
	}
	seeds := c.seeds()
	hostText := strings.Join(seeds, ",")
	noAuth := strings.EqualFold(strings.TrimSpace(c.MongoAuthMechanism), "NONE")
	user := strings.TrimSpace(c.User)
	if user != "" && !noAuth {
		info := url.User(user)
		if c.Password != "" {
			info = url.UserPassword(user, c.Password)
		}
		hostText = info.String() + "@" + hostText
	}
	path := "/"
	if database := strings.TrimSpace(c.Database); database != "" {
		path = "/" + url.PathEscape(database)
	}
	params := url.Values{}
	params.Set("connectTimeoutMS", strconv.Itoa(int(connectTimeout/time.Millisecond)))
	params.Set("serverSelectionTimeoutMS", strconv.Itoa(int(connectTimeout/time.Millisecond)))
	if user != "" && !noAuth {
		authSource := strings.TrimSpace(c.AuthSource)
		if authSource == "" {
			authSource = "admin"
		}
		params.Set("authSource", authSource)
	}
	if replicaSet := strings.TrimSpace(c.ReplicaSet); replicaSet != "" {
		params.Set("replicaSet", replicaSet)
	}
	if readPreference := strings.TrimSpace(c.ReadPreference); readPreference != "" {
		params.Set("readPreference", readPreference)
	}
	if mechanism := strings.TrimSpace(c.MongoAuthMechanism); mechanism != "" && !noAuth {
		params.Set("authMechanism", mechanism)
	}
	if c.tlsEnabled() {
		params.Set("tls", "true")
		if c.insecureTLS() {
			params.Set("tlsInsecure", "true")
		}
	}
	// A single host without replica-set discovery is the common saved form. The
	// driver otherwise tries to follow a topology it cannot reach.
	if !c.MongoSRV && strings.TrimSpace(c.ReplicaSet) == "" && !strings.EqualFold(strings.TrimSpace(c.Topology), "replica") {
		params.Set("directConnection", "true")
	}
	if err := applyConnectionParams(params, c.ConnectionParams); err != nil {
		return "", err
	}
	return scheme + "://" + hostText + path + "?" + params.Encode(), nil
}

func applyConnectionParams(params url.Values, raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	extra, err := url.ParseQuery(strings.TrimPrefix(raw, "?"))
	if err != nil {
		return newCodedError(codeInvalidArgument, "connection parameters are not a valid query string")
	}
	for key, values := range extra {
		if len(values) == 0 {
			continue
		}
		params.Del(key)
		for _, value := range values {
			params.Add(key, value)
		}
	}
	return nil
}

func (c ConnectionConfig) seeds() []string {
	candidates := c.Hosts
	if len(candidates) == 0 {
		candidates = []string{c.Host}
	}
	seeds := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		normalized := c.normalizeSeed(candidate)
		if normalized != "" {
			seeds = append(seeds, normalized)
		}
	}
	if len(seeds) == 0 {
		if c.MongoSRV {
			return []string{"localhost"}
		}
		return []string{fmt.Sprintf("localhost:%d", defaultMongoPort)}
	}
	return seeds
}

func (c ConnectionConfig) normalizeSeed(candidate string) string {
	seed := strings.TrimSpace(candidate)
	if seed == "" {
		return ""
	}
	if c.MongoSRV {
		// SRV forbids a port; keep the hostname as saved.
		host, _, err := net.SplitHostPort(seed)
		if err == nil && host != "" {
			return host
		}
		return strings.Trim(seed, "[]")
	}
	if _, _, err := net.SplitHostPort(seed); err == nil {
		return seed
	}
	// Bare host: add the saved or default port. Handle IPv6 literals.
	if strings.HasPrefix(seed, "[") && !strings.Contains(seed, "]:") {
		return seed + ":" + strconv.Itoa(c.port())
	}
	if strings.Contains(seed, ":") && !strings.Contains(seed, "]:") {
		// Already host:port for IPv4/hostname.
		return seed
	}
	return seed + ":" + strconv.Itoa(c.port())
}

func (c ConnectionConfig) port() int {
	if c.Port > 0 && c.Port <= 65535 {
		return c.Port
	}
	return defaultMongoPort
}

func (c ConnectionConfig) tlsEnabled() bool {
	if c.UseSSL || strings.TrimSpace(c.SSLCAPath) != "" || strings.TrimSpace(c.SSLCertPath) != "" {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(c.SSLMode)) {
	case "", "disable":
		return false
	default:
		return true
	}
}

func (c ConnectionConfig) insecureTLS() bool {
	switch strings.ToLower(strings.TrimSpace(c.SSLMode)) {
	case "skip-verify", "insecure", "allow":
		return true
	default:
		return false
	}
}

// TLSConfig builds a custom TLS configuration when the saved connection points
// at local CA/client material. A nil config means "let the driver handle it".
func (c ConnectionConfig) TLSConfig() (*tls.Config, error) {
	if !c.tlsEnabled() {
		return nil, nil
	}
	caPath := strings.TrimSpace(c.SSLCAPath)
	certPath := strings.TrimSpace(c.SSLCertPath)
	keyPath := strings.TrimSpace(c.SSLKeyPath)
	if caPath == "" && certPath == "" {
		return nil, nil
	}
	config := &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: c.insecureTLS()} //nolint:gosec // explicit user-selected SSL mode
	if caPath != "" {
		pem, err := os.ReadFile(caPath)
		if err != nil {
			return nil, wrapCodedError(codeInvalidArgument, "cannot read the configured SSL CA file", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, newCodedError(codeInvalidArgument, "the configured SSL CA file contains no certificates")
		}
		config.RootCAs = pool
	}
	if certPath != "" || keyPath != "" {
		if certPath == "" || keyPath == "" {
			return nil, newCodedError(codeInvalidArgument, "both the SSL client certificate and key are required")
		}
		certificate, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			return nil, wrapCodedError(codeInvalidArgument, "cannot load the configured SSL client certificate", err)
		}
		config.Certificates = []tls.Certificate{certificate}
	}
	return config, nil
}
