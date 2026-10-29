package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	plugin "github.com/Syngnat/GoNavi/sdk/go"
)

// accountCommandIDs is every command the account panel invokes, in manifest order.
var accountCommandIDs = []string{
	"account.login",
	"account.poll",
	"account.status",
	"account.keys",
	"account.groups",
	"account.listModels",
	"account.createKey",
	"account.updateKey",
	"account.deleteKey",
	"account.logout",
}

// accountCommandEffects pins the declared effect per command: launching a browser
// or clearing the stored session is a write, reading account data is a read.
var accountCommandEffects = map[string]string{
	"account.login":      "write",
	"account.poll":       "read",
	"account.status":     "read",
	"account.keys":       "read",
	"account.groups":     "read",
	"account.listModels": "read",
	"account.createKey":  "write",
	"account.updateKey":  "write",
	"account.deleteKey":  "write",
	"account.logout":     "write",
}

func findCommand(commands []plugin.Command, id string) (plugin.Command, bool) {
	for _, command := range commands {
		if command.ID == id {
			return command, true
		}
	}
	return plugin.Command{}, false
}

func readTestManifest(t *testing.T) *plugin.Manifest {
	t.Helper()
	raw, err := os.ReadFile("gonavi.plugin.json")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := plugin.ParseManifest(raw)
	if err != nil {
		t.Fatal(err)
	}
	return &manifest
}

func TestManifestDeclaresTheAccountPanel(t *testing.T) {
	manifest := readTestManifest(t)
	if manifest.Window == nil {
		t.Fatal("the account panel needs a window")
	}
	if manifest.Window.Entry != "ui/index.html" {
		t.Fatalf("unexpected window entry: %q", manifest.Window.Entry)
	}
	if manifest.Window.Title == "" || len(manifest.Window.Title) > 256 {
		t.Fatalf("unexpected window title: %q", manifest.Window.Title)
	}
	if manifest.Window.Width <= 0 || manifest.Window.Width > 8192 || manifest.Window.Height <= 0 || manifest.Window.Height > 8192 {
		t.Fatalf("unexpected window size: %dx%d", manifest.Window.Width, manifest.Window.Height)
	}
	// The declared entry must be the package-relative asset buildtool ships.
	if _, err := os.Stat(filepath.FromSlash(manifest.Window.Entry)); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ui/app.js", "ui/styles.css"} {
		if info, err := os.Stat(filepath.FromSlash(name)); err != nil || !info.Mode().IsRegular() {
			t.Fatalf("window asset %s is missing: %v", name, err)
		}
	}
}

func TestManifestDeclaresEveryAccountCommand(t *testing.T) {
	manifest := readTestManifest(t)
	if manifest.Contributes == nil {
		t.Fatal("missing contributions")
	}
	for _, id := range accountCommandIDs {
		command, ok := findCommand(manifest.Contributes.Commands, id)
		if !ok {
			t.Fatalf("command %q is not declared", id)
		}
		if command.Effect != accountCommandEffects[id] {
			t.Fatalf("command %q declares effect %q", id, command.Effect)
		}
		if err := plugin.ValidateCommand(command); err != nil {
			t.Fatalf("command %q: %v", id, err)
		}
		var schema map[string]any
		if err := json.Unmarshal(command.InputSchema, &schema); err != nil {
			t.Fatalf("command %q has an unparsable schema: %v", id, err)
		}
		if schema["type"] != "object" || schema["additionalProperties"] != false {
			t.Fatalf("command %q must accept a closed object argument", id)
		}
	}
}

// TestHandlersCoverEveryDeclaredCommand keeps the registry and the manifest in
// step: a declared command without a handler fails at runtime, not at install.
func TestHandlersCoverEveryDeclaredCommand(t *testing.T) {
	manifest := readTestManifest(t)
	handlers := newHandlers(nil, newTestService(t, stubSource{}, panelBaseURL))
	for _, command := range manifest.Contributes.Commands {
		if handlers[command.ID] == nil {
			t.Fatalf("command %q has no handler", command.ID)
		}
	}
	if len(handlers) != len(manifest.Contributes.Commands) {
		t.Fatalf("unexpected handler surface: %d handlers", len(handlers))
	}
}
