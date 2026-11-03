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
// The mechanism below stays even though it currently has no entries: a future
// field addition needs exactly this gate. Its one entry was
// navigation[].target -> "1.2.1", removed because that version never shipped, so
// the rule demanded a minimum no publisher could satisfy and rejected every
// package declaring the field (gonavi-ssh among them). The field's own legality
// is still checked by the shared SDK contract in artifact_contract.go; only the
// "declare a newer minimum" gate is empty.
var artifactHostRequirements = []artifactFieldRequirement{}

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
	if len(artifactHostRequirements) == 0 {
		return nil
	}
	minimum := normalizeArtifactHostVersion(manifest.MinHostVersion)
	for _, requirement := range artifactHostRequirements {
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

func normalizeArtifactHostVersion(value string) string {
	return strings.TrimPrefix(strings.TrimSpace(value), "v")
}
