package market

import (
	"fmt"
	"strings"
)

func validateArtifactConnectionActions(manifest artifactManifest) error {
	actions := manifest.Contributes.ConnectionActions
	if len(actions) > 16 {
		return fmt.Errorf("too many connection actions")
	}
	pages := make(map[string]bool, len(manifest.Contributes.Pages))
	for _, page := range manifest.Contributes.Pages {
		pages[page.ID] = true
	}
	seen := make(map[string]bool, len(actions))
	for _, action := range actions {
		if !artifactLocalIDPattern.MatchString(action.ID) || seen[action.ID] ||
			!artifactText(action.Title, 256) || !artifactOptionalText(action.Description, 4096) {
			return fmt.Errorf("invalid or duplicate connection action")
		}
		seen[action.ID] = true
		if len(action.ConnectionTypes) == 0 || len(action.ConnectionTypes) > 32 {
			return fmt.Errorf("connection action requires connection types")
		}
		types := make(map[string]bool, len(action.ConnectionTypes))
		for _, value := range action.ConnectionTypes {
			if !artifactConnectionType(value) || types[value] {
				return fmt.Errorf("invalid or duplicate connection action type")
			}
			types[value] = true
		}
		if len(action.Scopes) > 8 {
			return fmt.Errorf("too many connection action scopes")
		}
		scopes := make(map[string]bool, len(action.Scopes))
		for _, scope := range action.Scopes {
			switch scope {
			case "connection", "database", "collection":
			default:
				return fmt.Errorf("unsupported connection action scope")
			}
			if scopes[scope] {
				return fmt.Errorf("duplicate connection action scope")
			}
			scopes[scope] = true
		}
		switch action.Target.Type {
		case "window":
			if manifest.Window == nil || action.Target.Page != "" {
				return fmt.Errorf("window connection action requires a window")
			}
		case "page":
			if !artifactHasPermission(manifest.Permissions, "ui.pages") {
				return fmt.Errorf("page connection action requires ui.pages")
			}
			if !pages[action.Target.Page] {
				return fmt.Errorf("connection action references an undeclared page")
			}
		default:
			return fmt.Errorf("unsupported connection action target")
		}
	}
	return nil
}

func artifactHasPermission(permissions []string, permission string) bool {
	for _, value := range permissions {
		if value == permission {
			return true
		}
	}
	return false
}

func artifactConnectionType(value string) bool {
	if value == "" || len(value) > 128 || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || strings.ContainsRune("._:-", character) {
			continue
		}
		return false
	}
	return true
}
