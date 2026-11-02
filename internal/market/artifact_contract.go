package market

import (
	"fmt"

	plugin "github.com/Syngnat/GoNavi/sdk/go"
)

// The marketplace used to keep its own copy of the manifest field whitelist. That
// copy drifted from the host twice: it kept accepting configSchema after the host
// removed it, and it rejected panels[].html after the host added it. Both times
// the marketplace published packages a released host could not parse, which is
// exactly the failure that took the whole plugin runtime down before hosts began
// quarantining rejected records.
//
// Checking the same bytes with the SDK the host ships makes the field whitelist
// exist once. The SDK is pinned in go.mod, so a host contract change reaches the
// marketplace only when that pin is raised; keep it in step with the released
// host's tag.
func validateArtifactContract(raw []byte) error {
	// ParseManifest rejects unknown fields, trailing data and oversized input, so a
	// package accepted here is one the pinned host can decode.
	if _, err := plugin.ParseManifest(raw); err != nil {
		return fmt.Errorf("manifest does not match the host contract: %w", err)
	}
	return nil
}

// validateArtifactContractPages applies the SDK's page and navigation rules. This
// validator already checks those declarations in depth; the SDK call is the
// cross-check that the marketplace and the host agree on the shape, so a divergence
// surfaces here rather than on a user's machine.
func validateArtifactContractPages(raw []byte) error {
	manifest, err := plugin.ParseManifest(raw)
	if err != nil {
		return fmt.Errorf("manifest does not match the host contract: %w", err)
	}
	if err := plugin.ValidatePages(manifest); err != nil {
		return fmt.Errorf("page declarations do not match the host contract: %w", err)
	}
	return nil
}
