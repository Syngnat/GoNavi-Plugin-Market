package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	plugin "github.com/Syngnat/GoNavi/sdk/go"
)

// A top-level plugin icon is an author-facing manifest field the builder knows nothing
// about. marshalPackagedManifest must carry it through unchanged even when the pinned
// SDK predates the field, otherwise packaging silently drops the author's logo.
func TestMarshalPackagedManifestPreservesTopLevelIcon(t *testing.T) {
	var declared map[string]any
	if err := json.Unmarshal([]byte(`{
		"schemaVersion": 1, "apiVersion": 1, "id": "example.icon", "name": "Icon",
		"version": "1.0.0", "icon": "icons/plugin.png", "permissions": [],
		"contributes": {"commands": [{"id": "run", "title": "Run"}]}
	}`), &declared); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(declared)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "gonavi.plugin.json")
	if err = os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	encoded, err := marshalPackagedManifest(path, &plugin.Backend{Platforms: map[string]plugin.Entry{
		"linux/amd64": {Executable: "bin/plugin-linux-amd64", Runtime: "native"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	var packaged map[string]any
	if err = json.Unmarshal(encoded, &packaged); err != nil {
		t.Fatal(err)
	}
	if packaged["icon"] != "icons/plugin.png" {
		t.Fatalf("packaging dropped or rewrote the top-level icon: %v", packaged["icon"])
	}
	contributes, ok := packaged["contributes"].(map[string]any)
	if !ok || contributes["commands"] == nil {
		t.Fatal("packaging changed non-backend manifest declarations")
	}
}
