package shell

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func parseURI(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return parsed
}

func TestMongoURIBuildsSavedConnection(t *testing.T) {
	config := ConnectionConfig{
		Type:     "mongodb",
		Host:     "mongo.internal",
		Port:     27018,
		User:     "app",
		Password: "p@ss:word/1",
		Database: "shop",
	}
	raw, err := config.MongoURI()
	if err != nil {
		t.Fatalf("MongoURI: %v", err)
	}
	parsed := parseURI(t, raw)
	if parsed.Scheme != "mongodb" {
		t.Fatalf("scheme = %q", parsed.Scheme)
	}
	if parsed.Host != "mongo.internal:27018" {
		t.Fatalf("host = %q", parsed.Host)
	}
	if parsed.User.Username() != "app" {
		t.Fatalf("user = %q", parsed.User.Username())
	}
	password, _ := parsed.User.Password()
	if password != "p@ss:word/1" {
		t.Fatalf("password = %q", password)
	}
	if parsed.Path != "/shop" {
		t.Fatalf("path = %q", parsed.Path)
	}
	query := parsed.Query()
	if query.Get("authSource") != "admin" {
		t.Fatalf("authSource = %q", query.Get("authSource"))
	}
	if query.Get("directConnection") != "true" {
		t.Fatalf("directConnection = %q", query.Get("directConnection"))
	}
	if query.Get("connectTimeoutMS") != "10000" || query.Get("serverSelectionTimeoutMS") != "10000" {
		t.Fatalf("timeouts = %v", query)
	}
}

func TestMongoURISRVAndReplicaSet(t *testing.T) {
	srv := ConnectionConfig{Type: "mongodb", Host: "cluster.example.com", MongoSRV: true, Database: "app"}
	raw, err := srv.MongoURI()
	if err != nil {
		t.Fatalf("MongoURI: %v", err)
	}
	parsed := parseURI(t, raw)
	if parsed.Scheme != "mongodb+srv" {
		t.Fatalf("scheme = %q", parsed.Scheme)
	}
	if parsed.Host != "cluster.example.com" {
		t.Fatalf("host = %q", parsed.Host)
	}
	if parsed.Query().Get("directConnection") != "" {
		t.Fatalf("srv URI must not set directConnection: %v", parsed.Query())
	}

	replica := ConnectionConfig{
		Type: "mongodb", Hosts: []string{"m1:27017", "m2"}, ReplicaSet: "rs0", Topology: "replica",
	}
	raw, err = replica.MongoURI()
	if err != nil {
		t.Fatalf("MongoURI: %v", err)
	}
	parsed = parseURI(t, raw)
	if parsed.Host != "m1:27017,m2:27017" {
		t.Fatalf("hosts = %q", parsed.Host)
	}
	query := parsed.Query()
	if query.Get("replicaSet") != "rs0" {
		t.Fatalf("replicaSet = %q", query.Get("replicaSet"))
	}
	if query.Get("directConnection") != "" {
		t.Fatalf("replica URI must not set directConnection: %v", query)
	}
}

func TestMongoURIConnectionParamsOverride(t *testing.T) {
	config := ConnectionConfig{
		Type:             "mongodb",
		Host:             "db",
		Port:             27017,
		ConnectionParams: "connectTimeoutMS=5000&readPreferenceTags=dc:ny&tls=true",
	}
	raw, err := config.MongoURI()
	if err != nil {
		t.Fatalf("MongoURI: %v", err)
	}
	query := parseURI(t, raw).Query()
	if query.Get("connectTimeoutMS") != "5000" {
		t.Fatalf("connectTimeoutMS = %q", query.Get("connectTimeoutMS"))
	}
	if query.Get("readPreferenceTags") != "dc:ny" {
		t.Fatalf("readPreferenceTags = %q", query.Get("readPreferenceTags"))
	}
	if query.Get("tls") != "true" {
		t.Fatalf("tls = %q", query.Get("tls"))
	}
}

func TestMongoURIValidation(t *testing.T) {
	tests := []struct {
		name   string
		config ConnectionConfig
		want   string
	}{
		{name: "unsupported scheme", config: ConnectionConfig{URI: "postgres://db/app"}, want: codeInvalidArgument},
		{name: "bad params", config: ConnectionConfig{Type: "mongodb", Host: "db", ConnectionParams: "%zz"}, want: codeInvalidArgument},
		{name: "proxy", config: ConnectionConfig{Type: "mongodb", UseProxy: true}, want: codeUnsupportedTransport},
		{name: "wrong type", config: ConnectionConfig{Type: "mysql"}, want: codeUnsupportedConnection},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.config.MongoURI()
			if err == nil {
				t.Fatal("expected an error")
			}
			if code := errorCode(err); code != test.want {
				t.Fatalf("code = %q, want %q (%v)", code, test.want, err)
			}
		})
	}
}

func TestMongoURIExplicitURIIsAuthoritative(t *testing.T) {
	raw := "mongodb://user:secret@mongo:27017/app?replicaSet=rs0"
	config := ConnectionConfig{URI: raw, Host: "ignored"}
	got, err := config.MongoURI()
	if err != nil {
		t.Fatalf("MongoURI: %v", err)
	}
	if got != raw {
		t.Fatalf("URI = %q, want %q", got, raw)
	}
}

func TestForcedReadOnly(t *testing.T) {
	tests := []struct {
		name   string
		config ConnectionConfig
		want   bool
	}{
		{name: "legacy flag", config: ConnectionConfig{ReadOnly: true}, want: true},
		{name: "all restrictions", config: ConnectionConfig{Protection: Protection{
			RestrictDataEdit: true, RestrictStructureEdit: true,
			RestrictScriptExecution: true, RestrictDataImport: true,
		}}, want: true},
		{name: "partial restriction is not forced", config: ConnectionConfig{Protection: Protection{RestrictDataEdit: true}}, want: false},
		{name: "writable", config: ConnectionConfig{}, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.config.ForcedReadOnly(); got != test.want {
				t.Fatalf("ForcedReadOnly = %v, want %v", got, test.want)
			}
		})
	}
}

func TestTLSConfig(t *testing.T) {
	plain := ConnectionConfig{Type: "mongodb", Host: "db"}
	config, err := plain.TLSConfig()
	if err != nil || config != nil {
		t.Fatalf("plain TLSConfig = %v, %v", config, err)
	}

	insecure := ConnectionConfig{Type: "mongodb", Host: "db", UseSSL: true, SSLMode: "skip-verify"}
	config, err = insecure.TLSConfig()
	if err != nil || config != nil {
		t.Fatalf("skip-verify without custom material should defer to the driver: %v, %v", config, err)
	}
	raw, err := insecure.MongoURI()
	if err != nil {
		t.Fatalf("MongoURI: %v", err)
	}
	query := parseURI(t, raw).Query()
	if query.Get("tls") != "true" || query.Get("tlsInsecure") != "true" {
		t.Fatalf("TLS params = %v", query)
	}

	missingCA := ConnectionConfig{Type: "mongodb", Host: "db", SSLCAPath: "/nonexistent/ca.pem"}
	if _, err := missingCA.TLSConfig(); err == nil || errorCode(err) != codeInvalidArgument {
		t.Fatalf("missing CA error = %v", err)
	}
}

func TestDisplayHostHidesCredentials(t *testing.T) {
	config := ConnectionConfig{URI: "mongodb://user:secret@mongo.internal:27017/app"}
	if got := displayHost(config); got != "mongo.internal:27017" {
		t.Fatalf("displayHost = %q", got)
	}
	if strings.Contains(displayHost(config), "secret") {
		t.Fatal("displayHost leaked the password")
	}
}

func TestRedactSecrets(t *testing.T) {
	text := "parse mongodb://user:secret@host/db failed; also mongodb+srv://u:p@cluster/db"
	redacted := redactSecrets(text)
	if strings.Contains(redacted, "secret") || strings.Contains(redacted, "u:p@") {
		t.Fatalf("redacted = %q", redacted)
	}
	if !strings.Contains(redacted, "mongodb://user:***@host/db") {
		t.Fatalf("redacted = %q", redacted)
	}
}

func TestFailureEnvelopeShape(t *testing.T) {
	outcome := failure(newCodedError(codeReadOnly, "writes are disabled"))
	encoded, err := json.Marshal(outcome)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	text := string(encoded)
	if !strings.Contains(text, `"ok":false`) || !strings.Contains(text, `"code":"read_only"`) {
		t.Fatalf("envelope = %s", text)
	}
	if strings.Contains(text, `"data"`) {
		t.Fatalf("failure envelope must omit data: %s", text)
	}
}
