package market

import (
	"context"
	"testing"
)

// A released host rejects a manifest field it has never heard of, and a package
// accepted on that basis reaches users whose host cannot parse it. These tests
// pin the marketplace rule: using a newer field requires declaring a minimum
// host version that understands it.
func artifactCompatManifest(t *testing.T, minHostVersion any, navigation []any) artifactManifest {
	t.Helper()
	raw := artifactTestChange(t, func(fields map[string]any) {
		fields["window"] = map[string]any{"entry": "index.html"}
		if minHostVersion == nil {
			delete(fields, "minHostVersion")
		} else {
			fields["minHostVersion"] = minHostVersion
		}
		fields["contributes"] = map[string]any{"navigation": navigation}
	})
	manifest, err := decodeArtifactManifest([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}

// The minimum-version gate for navigation[].target was removed: it pointed at
// 1.2.1, a release that never shipped, so no publisher could satisfy it and every
// package declaring the field was rejected. These tests pin the behaviour that
// replaced it: the field itself is still validated by the shared SDK contract,
// and a navigation block without a target still publishes unchanged.
func TestArtifactNavigationTargetIsAcceptedWithoutMinimumGate(t *testing.T) {
	windowEntry := []any{artifactWindowTargetNavigation("window", "")}
	if err := validateArtifactHostCompatibility(artifactCompatManifest(t, "1.2.0", windowEntry)); err != nil {
		t.Fatalf("navigation target was rejected after the gate was removed: %v", err)
	}
	if err := validateArtifactHostCompatibility(artifactCompatManifest(t, nil, windowEntry)); err != nil {
		t.Fatalf("navigation target without a minimum was rejected: %v", err)
	}
}

func TestArtifactNavigationWithoutTargetKeepsOldHosts(t *testing.T) {
	// A navigation entry that omits target predates the field and must stay
	// publishable without any minimum, so old packages are not retroactively
	// broken.
	legacy := []any{map[string]any{"id": "nav", "title": "N", "page": "p", "location": "tools"}}
	if err := validateArtifactHostCompatibility(artifactCompatManifest(t, nil, legacy)); err != nil {
		t.Fatalf("legacy navigation without target was rejected: %v", err)
	}
	if err := validateArtifactHostCompatibility(artifactCompatManifest(t, "1.0.0", legacy)); err != nil {
		t.Fatalf("legacy navigation with an older minimum was rejected: %v", err)
	}
}

// The field's own legality is still enforced, just by the shared SDK contract
// rather than by a hand-written marketplace rule.
func TestValidatePackageRejectsInvalidNavigationTarget(t *testing.T) {
	manifest := artifactTestChange(t, func(fields map[string]any) {
		fields["permissions"] = []any{"ui.html-panel"}
		fields["window"] = map[string]any{"entry": "index.html"}
		fields["contributes"] = map[string]any{
			"navigation": []any{map[string]any{"id": "nav", "title": "N", "target": "browser", "location": "tools"}},
		}
	})
	entries := append(artifactTestEntries(manifest), artifactTestEntry{name: "index.html", body: []byte("<!doctype html><title>W</title>")})
	filename := artifactTestWrite(t, ".zip", artifactTestZIP(t, entries))
	if _, err := ValidatePackage(context.Background(), filename); err == nil {
		t.Fatal("an unsupported navigation target was published")
	}
}

func TestValidatePackageRejectsRemovedConfigSchema(t *testing.T) {
	// configSchema was dropped from the host contract. Every host now treats it as
	// an unknown field, so a package that declares it must not publish.
	manifest := artifactTestChange(t, func(fields map[string]any) {
		fields["configSchema"] = map[string]any{"type": "object"}
	})
	filename := artifactTestWrite(t, ".zip", artifactTestZIP(t, artifactTestEntries(manifest)))
	if _, err := ValidatePackage(context.Background(), filename); err == nil {
		t.Fatal("package declaring the removed configSchema was published")
	}
}

// The SDK owns the field whitelist, so a field the pinned host cannot decode dies
// here instead of on a user's machine. configSchema is the real regression: the
// host removed it and this validator kept accepting it.
func TestContractCheckRejectsFieldTheHostDropped(t *testing.T) {
	err := validateArtifactContract([]byte(`{"schemaVersion":1,"apiVersion":1,"id":"a.b","name":"N","version":"1.0.0","permissions":[],"configSchema":{"type":"object"}}`))
	if err == nil {
		t.Fatal("a field the pinned host dropped was accepted")
	}
}

func TestContractCheckAcceptsCurrentHostShape(t *testing.T) {
	// panels[].html is in the contract now; the whitelist must follow the host, not
	// a stale copy.
	raw := []byte(`{"schemaVersion":1,"apiVersion":1,"id":"a.b","name":"N","version":"1.0.0","permissions":["ui.html-panel"],` +
		`"contributes":{"panels":[{"id":"p","title":"P","html":{"source":"<p>x</p>"}}]}}`)
	if err := validateArtifactContract(raw); err != nil {
		t.Fatalf("a declaration the host accepts was rejected: %v", err)
	}
}
