package store

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }

func contains(s, sub string) bool { return strings.Contains(s, sub) }

func newTestStore(t *testing.T) *Store {
	t.Helper()
	return Open(filepath.Join(t.TempDir(), "connections.json"))
}

func TestSaveListDelete(t *testing.T) {
	s := newTestStore(t)
	saved, err := s.Save(Profile{Name: "prod", Host: "10.0.0.1", Port: 22, Username: "deploy", AuthType: AuthPassword, Password: "secret"})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if saved.ID == "" || saved.CreatedAt == "" {
		t.Fatalf("id/timestamps not assigned: %+v", saved)
	}
	list, err := s.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v %d", err, len(list))
	}
	if list[0].Password != "secret" {
		t.Fatalf("store must keep the password for dialing")
	}

	// Update keeps the id and creation time.
	saved.Name = "prod-web"
	updated, err := s.Save(saved)
	if err != nil || updated.ID != saved.ID || updated.CreatedAt != saved.CreatedAt {
		t.Fatalf("update: %+v", updated)
	}

	if err := s.Delete(saved.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if list, _ := s.List(); len(list) != 0 {
		t.Fatalf("list after delete = %d", len(list))
	}
	if err := s.Delete("missing"); err == nil {
		t.Fatalf("delete of missing id should fail")
	}
}

func TestValidate(t *testing.T) {
	s := newTestStore(t)
	cases := []Profile{
		{Name: "", Host: "h", Port: 22, Username: "u", AuthType: AuthPassword, Password: "p"},
		{Name: "n", Host: "", Port: 22, Username: "u", AuthType: AuthPassword, Password: "p"},
		{Name: "n", Host: "h", Port: 0, Username: "u", AuthType: AuthPassword, Password: "p"},
		{Name: "n", Host: "h", Port: 22, Username: "", AuthType: AuthPassword, Password: "p"},
		{Name: "n", Host: "h", Port: 22, Username: "u", AuthType: AuthPassword, Password: ""},
		{Name: "n", Host: "h", Port: 22, Username: "u", AuthType: AuthKey, KeyPath: ""},
		{Name: "n", Host: "h", Port: 22, Username: "u", AuthType: "agent"},
		{Name: "n", Host: "h with space", Port: 22, Username: "u", AuthType: AuthPassword, Password: "p"},
	}
	for i, p := range cases {
		if _, err := s.Save(p); err == nil {
			t.Errorf("case %d: expected validation error", i)
		}
	}
}

func TestSetFingerprintTOFU(t *testing.T) {
	s := newTestStore(t)
	saved, _ := s.Save(Profile{Name: "a", Host: "h", Port: 22, Username: "u", AuthType: AuthPassword, Password: "p"})
	if err := s.SetFingerprint(saved.ID, "SHA256:abc"); err != nil {
		t.Fatalf("set: %v", err)
	}
	// First value wins; later calls must not overwrite.
	if err := s.SetFingerprint(saved.ID, "SHA256:evil"); err != nil {
		t.Fatalf("set again: %v", err)
	}
	got, ok, _ := s.Get(saved.ID)
	if !ok || got.Fingerprint != "SHA256:abc" {
		t.Fatalf("fingerprint = %q", got.Fingerprint)
	}
}

func TestMaskProfiles(t *testing.T) {
	masked := MaskProfiles([]Profile{{
		ID: "id", Name: "n", Host: "h", Port: 22, Username: "u",
		AuthType: AuthKey, KeyPath: "/keys/id", KeyPassphrase: "pp", Password: "pw", Fingerprint: "SHA256:x",
	}})
	if len(masked) != 1 {
		t.Fatalf("masked len = %d", len(masked))
	}
	m := masked[0]
	if !m.HasPassword || !m.HasKeyPass {
		t.Fatalf("presence flags wrong: %+v", m)
	}
	if m.Fingerprint != "SHA256:x" || m.KeyPath != "/keys/id" {
		t.Fatalf("non-secret fields lost: %+v", m)
	}
	// MaskedProfile must not carry secret fields at all.
	if anyJSONContains(m, "pw") || anyJSONContains(m, "pp") {
		t.Fatalf("masked profile leaks secrets")
	}
}

func anyJSONContains(v any, needle string) bool {
	data, _ := jsonMarshal(v)
	return contains(string(data), needle)
}
