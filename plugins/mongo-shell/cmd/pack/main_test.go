package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestApplyBackendPreservesUnknownContributions(t *testing.T) {
	raw := []byte(`{
		"schemaVersion": 1,
		"apiVersion": 1,
		"id": "com.gonavi.mongo-shell",
		"name": "MongoShell",
		"version": "1.0.0",
		"permissions": ["host.connections.credentials"],
		"backend": {"default": {"executable": "bin/plugin", "runtime": "native"}},
		"contributes": {
			"commands": [{"id": "run", "title": "Run"}],
			"connectionActions": [{"id": "open-shell", "title": "MongoShell", "target": {"type": "window"}}]
		}
	}`)
	encoded, err := applyBackend(raw, []target{{goos: "darwin", goarch: "arm64"}})
	if err != nil {
		t.Fatalf("applyBackend: %v", err)
	}
	text := string(encoded)
	for _, want := range []string{
		`"connectionActions"`,
		`"open-shell"`,
		`"host.connections.credentials"`,
		`"darwin/arm64"`,
		`"bin/plugin-darwin-arm64"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("manifest lost %s:\n%s", want, text)
		}
	}
	if strings.Contains(text, `"default"`) {
		t.Fatalf("manifest kept the template backend.default:\n%s", text)
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decoded manifest is not JSON: %v", err)
	}
	backend := decoded["backend"].(map[string]any)
	platforms := backend["platforms"].(map[string]any)
	entry := platforms["darwin/arm64"].(map[string]any)
	if entry["runtime"] != "native" {
		t.Fatalf("platform entry = %v", entry)
	}
	contributes := decoded["contributes"].(map[string]any)
	if _, ok := contributes["connectionActions"]; !ok {
		t.Fatalf("contributes = %v", contributes)
	}
}

func TestApplyBackendRejectsIncompleteManifest(t *testing.T) {
	raw := []byte(`{"schemaVersion":1,"apiVersion":1,"name":"X","version":"1.0.0"}`)
	if _, err := applyBackend(raw, []target{{goos: "darwin", goarch: "arm64"}}); err == nil {
		t.Fatal("expected a missing id error")
	}
	if _, err := applyBackend([]byte(`null`), []target{{goos: "darwin", goarch: "arm64"}}); err == nil {
		t.Fatal("expected a null manifest error")
	}
	if _, err := applyBackend([]byte(`not json`), []target{{goos: "darwin", goarch: "arm64"}}); err == nil {
		t.Fatal("expected a decode error")
	}
}

func TestParseTargets(t *testing.T) {
	targets, err := parseTargets("darwin/arm64")
	if err != nil || len(targets) != 1 || targets[0].key() != "darwin/arm64" {
		t.Fatalf("targets = %v, %v", targets, err)
	}
	targets, err = parseTargets("")
	if err != nil || len(targets) != len(defaultTargets) {
		t.Fatalf("default targets = %v, %v", targets, err)
	}
	if _, err := parseTargets("darwin"); err == nil {
		t.Fatal("expected an invalid platform error")
	}
	if _, err := parseTargets("darwin/"); err == nil {
		t.Fatal("expected an empty arch error")
	}
}

func TestTargetExecutableNames(t *testing.T) {
	if got := (target{goos: "darwin", goarch: "arm64"}).executable(); got != "bin/plugin-darwin-arm64" {
		t.Fatalf("darwin executable = %q", got)
	}
	if got := (target{goos: "windows", goarch: "amd64"}).executable(); got != "bin/plugin-windows-amd64.exe" {
		t.Fatalf("windows executable = %q", got)
	}
}
