package market

import (
	"context"
	"strings"
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

func TestArtifactNavigationTargetRequiresParseableHost(t *testing.T) {
	windowEntry := []any{artifactWindowTargetNavigation("window", "")}

	// An absent minimum cannot promise the field is understood.
	if err := validateArtifactHostCompatibility(artifactCompatManifest(t, nil, windowEntry)); err == nil {
		t.Fatal("navigation target without minHostVersion was accepted")
	}
	// A minimum below the release that added the field is the dangerous case:
	// the package publishes, the latest channel installs, and the host cannot
	// parse the navigation entry.
	below := "1.2.0"
	if err := validateArtifactHostCompatibility(artifactCompatManifest(t, below, windowEntry)); err == nil {
		t.Fatalf("navigation target with minHostVersion %s was accepted", below)
	}
	// The first release that understands the field, and anything newer, is fine.
	for _, accepted := range []string{artifactMinHostNavigationTarget, "1.2.1", "1.3.0", "v1.2.1"} {
		if err := validateArtifactHostCompatibility(artifactCompatManifest(t, accepted, windowEntry)); err != nil {
			t.Fatalf("navigation target with minHostVersion %s was rejected: %v", accepted, err)
		}
	}
}

func TestArtifactPageTargetAlsoRequiresParseableHost(t *testing.T) {
	// The target key itself is what an older host rejects, so an explicit page
	// target needs the same minimum as a window target.
	pageEntry := []any{map[string]any{"id": "nav", "title": "N", "target": "page", "location": "tools"}}
	if err := validateArtifactHostCompatibility(artifactCompatManifest(t, "1.2.0", pageEntry)); err == nil {
		t.Fatal("explicit page target below the field minimum was accepted")
	}
	if err := validateArtifactHostCompatibility(artifactCompatManifest(t, artifactMinHostNavigationTarget, pageEntry)); err != nil {
		t.Fatalf("explicit page target at the field minimum was rejected: %v", err)
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

func TestValidatePackageRejectsWindowTargetWithoutMinimum(t *testing.T) {
	manifest := artifactTestChange(t, func(fields map[string]any) {
		fields["window"] = map[string]any{"entry": "index.html"}
		delete(fields, "minHostVersion")
		fields["contributes"] = map[string]any{
			"navigation": []any{artifactWindowTargetNavigation("window", "")},
		}
	})
	entries := append(artifactTestEntries(manifest), artifactTestEntry{name: "index.html", body: []byte("<!doctype html><title>W</title>")})
	filename := artifactTestWrite(t, ".zip", artifactTestZIP(t, entries))
	_, err := ValidatePackage(context.Background(), filename)
	if err == nil {
		t.Fatal("window target package without minHostVersion was published")
	}
	if !strings.Contains(err.Error(), "minHostVersion") {
		t.Fatalf("rejection did not name the missing minimum: %v", err)
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
