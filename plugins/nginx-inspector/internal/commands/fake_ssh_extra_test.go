package commands

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/sshx"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/store"
)

func marshalArgs(t *testing.T, payload map[string]any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	return data
}

func TestSaveConnectionPreservesSecretsAndFingerprint(t *testing.T) {
	st := store.Open(filepath.Join(t.TempDir(), "connections.json"))
	runtime := &Runtime{store: st, pool: sshx.NewPool(st), info: map[string]*HostInfo{}}
	ctx := context.Background()
	base := map[string]any{
		"name": "prod", "host": "example.com", "port": 22,
		"username": "deploy", "authType": store.AuthPassword, "password": "secret",
	}
	if _, err := runtime.saveConnection(ctx, marshalArgs(t, base)); err != nil {
		t.Fatalf("initial save: %v", err)
	}
	profiles, err := st.List()
	if err != nil || len(profiles) != 1 {
		t.Fatalf("list: %v %d", err, len(profiles))
	}
	id := profiles[0].ID
	if err := st.SetFingerprint(id, "SHA256:abc"); err != nil {
		t.Fatalf("set fingerprint: %v", err)
	}
	rename := map[string]any{
		"id": id, "name": "renamed", "host": "example.com", "port": 22,
		"username": "deploy", "authType": store.AuthPassword,
	}
	if _, err := runtime.saveConnection(ctx, marshalArgs(t, rename)); err != nil {
		t.Fatalf("rename save: %v", err)
	}
	stored, ok, err := st.Get(id)
	if err != nil || !ok {
		t.Fatalf("get after rename: %v %v", ok, err)
	}
	if stored.Name != "renamed" || stored.Password != "secret" || stored.Fingerprint != "SHA256:abc" {
		t.Fatalf("stored after rename = %+v", stored)
	}
	pin := map[string]any{
		"id": id, "name": "renamed", "host": "example.com", "port": 22,
		"username": "deploy", "authType": store.AuthPassword, "fingerprint": "SHA256:new",
	}
	if _, err := runtime.saveConnection(ctx, marshalArgs(t, pin)); err != nil {
		t.Fatalf("fingerprint save: %v", err)
	}
	stored, _, _ = st.Get(id)
	if stored.Password != "secret" || stored.Fingerprint != "SHA256:new" {
		t.Fatalf("stored after fingerprint pin = %+v", stored)
	}
}

func TestAccessStatsGzipIntegration(t *testing.T) {
	files := map[string][]byte{"/var/log/nginx/access.log": []byte("compressed")}
	base := fileCommands(files)
	handler := func(command string) (string, string, int) {
		if strings.HasPrefix(command, "head -c 2 ") {
			return "1f 8b", "", 0
		}
		return base(command)
	}
	host, port := startFakeSSH(t, handler)
	runtime, connectionID := newFakeRuntime(t, host, port)
	result, err := runtime.accessStats(context.Background(), connectionArgs(t, connectionID, "bytes", int64(1024)))
	if err != nil {
		t.Fatalf("access.stats: %v", err)
	}
	stats := result.(*AccessStats)
	if !stats.GzipUnsupported {
		t.Fatalf("gzip log must be reported as unsupported")
	}
	if stats.WindowBytes != 0 || stats.Total != 0 || len(stats.Statuses) != 0 {
		t.Fatalf("gzip stats should be empty: %+v", stats)
	}
	if stats.RequestedBytes != 1024 {
		t.Fatalf("requestedBytes = %d", stats.RequestedBytes)
	}
}

func TestUpstreamsCheckIntegration(t *testing.T) {
	config := `http {
    upstream primary {
        server 127.0.0.1:1 down;
    }
    upstream pooled {
        server unix:/tmp/app.sock;
        server 127.0.0.1:2 backup;
    }
}`
	host, port := startFakeSSH(t, fileCommands(map[string][]byte{"/etc/nginx/nginx.conf": []byte(config)}))
	runtime, connectionID := newFakeRuntime(t, host, port)
	result, err := runtime.upstreamsCheck(context.Background(), connectionArgs(t, connectionID))
	if err != nil {
		t.Fatalf("upstreams.check: %v", err)
	}
	check, ok := result.(*UpstreamCheck)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	if check.Backup != 1 {
		t.Fatalf("backup = %d", check.Backup)
	}
	if check.Down != 0 {
		t.Fatalf("backup or disabled members must not count as down: %+v", check)
	}
	if check.Skipped != 1 {
		t.Fatalf("unix socket should be skipped exactly once: %+v", check)
	}
	var disabled, backup *UpstreamMemberCheck
	for i := range check.Upstreams {
		for j := range check.Upstreams[i].Servers {
			member := &check.Upstreams[i].Servers[j]
			if member.Disabled {
				disabled = member
			}
			if member.Backup {
				backup = member
			}
		}
	}
	if disabled == nil || disabled.Alive != nil {
		t.Fatalf("down member should be marked disabled and never probed: %+v", disabled)
	}
	if backup == nil || backup.Alive == nil || *backup.Alive {
		t.Fatalf("backup member should be probed and reported unreachable: %+v", backup)
	}
}
