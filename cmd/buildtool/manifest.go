package main

import (
	"encoding/json"
	"fmt"
	"os"

	plugin "github.com/Syngnat/GoNavi/sdk/go"
)

// marshalPackagedManifest preserves declarations newer than the pinned SDK.
// Only the generated backend platform entries belong to the package builder.
func marshalPackagedManifest(path string, backend *plugin.Backend) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var manifest map[string]json.RawMessage
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	if manifest == nil {
		return nil, fmt.Errorf("manifest must be a JSON object")
	}
	encoded, err := json.Marshal(backend)
	if err != nil {
		return nil, fmt.Errorf("encode backend: %w", err)
	}
	manifest["backend"] = encoded
	return json.MarshalIndent(manifest, "", "  ")
}
