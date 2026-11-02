package market

import (
	"fmt"
	"strings"

	"golang.org/x/mod/semver"
)

// Contributed manifest fields enter the host contract one release at a time, and
// a released host rejects a manifest that uses a field it has never heard of.
// The marketplace is the only party that sees a package before both audiences:
// a developer on the dev channel (which accepts the newest source) and a user on
// the latest channel (which is one or more releases behind). Accepting a package
// without a minimum host version therefore ships it to a host that cannot parse
// it; before the host quarantined such records that failure took the whole
// plugin runtime down.
//
// Every field added to the host contract after v1 needs an entry here, naming
// the first released version that understands it. Publishing a package that uses
// the field without declaring at least that minimum is rejected.
const (
	// navigation[].target ("page" default / "window") landed after v1.2.0.
	artifactMinHostNavigationTarget = "1.2.1"
)

// artifactFieldRequirement is one contract addition and the first release that
// accepts it.
type artifactFieldRequirement struct {
	// field names the manifest path for the rejection message.
	field string
	// minimum is the first released host version that understands the field.
	minimum string
}

// validateArtifactHostCompatibility rejects declarations that a released host
// could not parse. It runs after the manifest is structurally valid, so every
// field it inspects already passed its own shape and value checks.
func validateArtifactHostCompatibility(manifest artifactManifest) error {
	var requirements []artifactFieldRequirement
	if usesArtifactNavigationTarget(manifest) {
		requirements = append(requirements, artifactFieldRequirement{
			field:   "contributes.navigation[].target",
			minimum: artifactMinHostNavigationTarget,
		})
	}
	if len(requirements) == 0 {
		return nil
	}
	minimum := normalizeArtifactHostVersion(manifest.MinHostVersion)
	for _, requirement := range requirements {
		if minimum == "" {
			return fmt.Errorf("%s requires minHostVersion %s or newer", requirement.field, requirement.minimum)
		}
		if !semver.IsValid("v" + minimum) {
			return fmt.Errorf("invalid minimum host version")
		}
		if semver.Compare("v"+minimum, "v"+requirement.minimum) < 0 {
			return fmt.Errorf("%s requires minHostVersion %s or newer", requirement.field, requirement.minimum)
		}
	}
	return nil
}

// usesArtifactNavigationTarget reports whether any navigation entry declares a
// target at all. An empty target is the pre-existing page default, so only the
// presence of the key needs a newer host; page and window both do, because the
// field itself is what an older host rejects.
func usesArtifactNavigationTarget(manifest artifactManifest) bool {
	for _, item := range manifest.Contributes.Navigation {
		if item.Target != "" {
			return true
		}
	}
	return false
}

func normalizeArtifactHostVersion(value string) string {
	return strings.TrimPrefix(strings.TrimSpace(value), "v")
}
