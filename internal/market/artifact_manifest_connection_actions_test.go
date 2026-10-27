package market

import (
	"encoding/json"
	"strings"
	"testing"
)

func connectionActionManifest(action string) artifactManifest {
	raw := `{"schemaVersion":1,"apiVersion":1,"id":"com.example.mongo","name":"Mongo","version":"1.0.0","permissions":["host.connections.credentials"],"window":{"entry":"ui/index.html"},"contributes":` + action + `}`
	manifest, err := decodeArtifactManifest([]byte(raw))
	if err != nil {
		panic(err)
	}
	return manifest
}

func TestArtifactConnectionActionsAreValidated(t *testing.T) {
	valid := connectionActionManifest(`{"connectionActions":[{"id":"open","title":"MongoShell","connectionTypes":["mongodb"],"scopes":["connection","database","collection"],"target":{"type":"window"}}]}`)
	if err := validateArtifactContributions(valid); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(valid)
	if err != nil || !strings.Contains(string(encoded), "connectionActions") {
		t.Fatalf("connection actions are not preserved: %s", encoded)
	}
	cases := []string{
		`{"connectionActions":[{"id":"open","title":"Open","connectionTypes":["MongoDB"],"target":{"type":"window"}}]}`,
		`{"connectionActions":[{"id":"open","title":"Open","connectionTypes":["mongodb"],"scopes":["schema"],"target":{"type":"window"}}]}`,
		`{"connectionActions":[{"id":"open","title":"Open","connectionTypes":["mongodb"],"target":{"type":"page","page":"missing"}}]}`,
		`{"connectionActions":[{"id":"open","title":"Open","connectionTypes":["mongodb"],"target":{"type":"window"}},{"id":"open","title":"Again","connectionTypes":["mongodb"],"target":{"type":"window"}}]}`,
	}
	for _, action := range cases {
		if err := validateArtifactContributions(connectionActionManifest(action)); err == nil {
			t.Fatalf("invalid connection action accepted: %s", action)
		} else {
			t.Logf("expected rejection: %v", err)
		}
	}
	page := connectionActionManifest(`{"pages":[{"id":"shell","title":"Shell","body":{"type":"text","text":"ok"}}],"connectionActions":[{"id":"open","title":"Open","connectionTypes":["mongodb"],"target":{"type":"page","page":"shell"}}]}`)
	if err := validateArtifactContributions(page); err == nil {
		// page targets also require ui.pages; the host manifest validator owns that check.
		t.Fatal("page target without permission accepted")
	}
}
