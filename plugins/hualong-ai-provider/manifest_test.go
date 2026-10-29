package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	plugin "github.com/Syngnat/GoNavi/sdk/go"
)

// expectedModelIDs is the live GET /v1/models result from the 華龍算力 gateway, verbatim.
var expectedModelIDs = []string{
	"deepseek-v4-pro",
	"deepseek-v4-flash",
	"deepseek-v4.1-flash",
	"glm-5.3",
	"glm-5.3-flash",
	"kimi-k3",
	"minimax-m3",
	"glm-5v-turbo",
	"kimi-k2.6",
	"kimi-k2.7",
	"kimi-k3.1-pro",
	"minimax-m3.1-flash",
}

func TestManifestParsesAndMatchesExecutable(t *testing.T) {
	raw, err := os.ReadFile("gonavi.plugin.json")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := plugin.ParseManifest(raw)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ID != "com.gonavi.hualong-ai" || manifest.Name != providerName || manifest.Version != "1.1.0" {
		t.Fatal("incorrect identity")
	}
	if manifest.Author != "GoNavi" || manifest.License != "Apache-2.0" || manifest.APIVersion != 1 || manifest.SchemaVersion != 1 {
		t.Fatal("incorrect metadata")
	}
	if len(manifest.Permissions) != 1 || manifest.Permissions[0] != "ai.providers" {
		t.Fatal("incorrect permissions")
	}
	if manifest.Backend == nil || manifest.Backend.Default == nil || manifest.Backend.Default.Executable != "bin/plugin" || manifest.Backend.Default.Runtime != "native" {
		t.Fatal("manifest does not declare the native executable")
	}
	if manifest.Window == nil || manifest.Window.Entry != "ui/index.html" {
		t.Fatal("the account panel must declare a window entry")
	}
	contributes := manifest.Contributes
	if contributes == nil || len(contributes.AIProviders) != 1 || len(contributes.Commands) != 1+len(accountCommandIDs) {
		t.Fatal("unexpected extension surface")
	}
	providerCommand, ok := findCommand(contributes.Commands, "provider")
	if !ok || providerCommand.Effect != "read" {
		t.Fatal("the provider command must be declared as a read command")
	}
	declaration := contributes.AIProviders[0]
	if declaration.ID != providerID || declaration.Name != providerName || declaration.Command != providerCommand.ID {
		t.Fatal("provider declaration does not match the registered command")
	}
	if declaration.Icon != "icons/provider.png" {
		t.Fatalf("unexpected icon: %q", declaration.Icon)
	}
	if !declaration.SupportsTools || declaration.SupportsImages {
		t.Fatal("provider capabilities do not match the registration")
	}
	if !reflect.DeepEqual(declaration.Models, providerModels()) {
		t.Fatal("manifest does not match the registered models")
	}
	if err = plugin.ValidateAIProvider(declaration); err != nil {
		t.Fatal(err)
	}
}

func TestManifestDeclaresEveryGatewayModel(t *testing.T) {
	raw, err := os.ReadFile("gonavi.plugin.json")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := plugin.ParseManifest(raw)
	if err != nil {
		t.Fatal(err)
	}
	models := manifest.Contributes.AIProviders[0].Models
	if len(models) != len(expectedModelIDs) {
		t.Fatalf("expected %d models, got %d", len(expectedModelIDs), len(models))
	}
	seen := make(map[string]bool, len(models))
	for index, model := range models {
		if model.ID != expectedModelIDs[index] {
			t.Fatalf("model %d: expected %q, got %q", index, expectedModelIDs[index], model.ID)
		}
		if seen[model.ID] {
			t.Fatalf("duplicate model %q", model.ID)
		}
		seen[model.ID] = true
		if model.ContextWindow != contextWindow {
			t.Fatalf("model %q has an unexpected context window", model.ID)
		}
		want := maxOutputTokens
		if model.ID == "glm-5v-turbo" {
			want = visionTokens
		}
		if model.MaxOutputTokens != want {
			t.Fatalf("model %q has an unexpected output cap", model.ID)
		}
	}
}

func TestIconIsABoundedPackageRaster(t *testing.T) {
	info, err := os.Stat(filepath.FromSlash("icons/provider.png"))
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() || info.Size() == 0 || info.Size() > 64<<10 {
		t.Fatalf("icon must be a non-empty regular file of at most 64 KiB, got %d bytes", info.Size())
	}
	raw, err := os.ReadFile(filepath.FromSlash("icons/provider.png"))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 8 || string(raw[1:4]) != "PNG" {
		t.Fatal("icon is not a PNG image")
	}
}
