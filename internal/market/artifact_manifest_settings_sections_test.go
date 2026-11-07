package market

import (
	"encoding/json"
	"strings"
	"testing"
)

func settingsSectionManifest(contributes string) (artifactManifest, []byte) {
	raw := []byte(`{"schemaVersion":1,"apiVersion":1,"id":"com.example.background","name":"Background","version":"1.0.0","minHostVersion":"1.2.3","permissions":["ui.pages","ui.html-panel"],"contributes":` + contributes + `}`)
	manifest, err := decodeArtifactManifest(raw)
	if err != nil {
		panic(err)
	}
	return manifest, raw
}

func TestArtifactSettingsSectionsAreDecoded(t *testing.T) {
	contributes := `{"pages":[{"id":"background","title":"Background","html":{"source":"<p>ok</p>"}}],"settingsSections":[{"id":"background-settings","slot":"appearance.background","page":"background","order":0}]}`
	manifest, raw := settingsSectionManifest(contributes)
	if err := validateArtifactContract(raw); err != nil {
		t.Fatal(err)
	}
	if err := validateArtifactContributions(manifest); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(manifest)
	if err != nil || !strings.Contains(string(encoded), "settingsSections") {
		t.Fatalf("settings sections are not preserved: %s", encoded)
	}
}

func TestArtifactSettingsSectionsRejectUnknownSlotAndMissingPage(t *testing.T) {
	cases := []string{
		`{"pages":[{"id":"background","title":"Background","html":{"source":"<p>ok</p>"}}],"settingsSections":[{"id":"s","slot":"appearance.unknown","page":"background"}]}`,
		`{"pages":[{"id":"background","title":"Background","html":{"source":"<p>ok</p>"}}],"settingsSections":[{"id":"s","slot":"appearance.background","page":"missing"}]}`,
	}
	for _, contributes := range cases {
		_, raw := settingsSectionManifest(contributes)
		if err := validateArtifactContract(raw); err == nil {
			t.Fatalf("invalid settings section accepted: %s", contributes)
		}
	}
}
