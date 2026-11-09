package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The host serves the window document at "/" and only maps /plugin-assets/<package path>
// to package files. Relative references resolve against "/" and 404, which leaves the
// window as a bare HTML skeleton (the 1.0.2 blank-window bug).
func TestWindowEntryUsesHostAssetPaths(t *testing.T) {
	raw, err := os.ReadFile("ui/index.html")
	if err != nil {
		t.Fatalf("read window entry: %v", err)
	}
	body := string(raw)
	if strings.Contains(strings.ToLower(body), "content-security-policy") {
		t.Fatal("ui/index.html must not declare its own CSP: the host injects the policy and an inline bootstrap script")
	}
	references := regexp.MustCompile(`(?:href|src)="([^"]+)"`).FindAllStringSubmatch(body, -1)
	if len(references) == 0 {
		t.Fatal("window entry has no href/src references")
	}
	for _, match := range references {
		reference := match[1]
		if reference == "" || strings.HasPrefix(reference, "#") {
			continue
		}
		if !strings.HasPrefix(reference, "/plugin-assets/") {
			t.Errorf("reference %q is unreachable in the plugin window; use /plugin-assets/<package path>", reference)
			continue
		}
		asset := strings.TrimPrefix(reference, "/plugin-assets/")
		if _, err := os.Stat(asset); err != nil {
			t.Errorf("referenced asset %q does not exist in the package: %v", asset, err)
		}
	}
}
