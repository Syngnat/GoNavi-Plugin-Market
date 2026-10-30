package market

import (
	"context"
	"strings"
	"testing"
)

// TestArtifactPluginIconAcceptsDeclaredRasterFile pins the shared icon contract:
// a top-level manifest icon is a package-relative raster path that must resolve to
// an exact regular package file no larger than the host's 5 MiB read budget.
func TestArtifactPluginIconAcceptsDeclaredRasterFile(t *testing.T) {
	cases := []struct {
		name string
		icon string
		body []byte
	}{
		{name: "png", icon: "icons/plugin.png", body: []byte("\x89PNG\r\n\x1a\n")},
		{name: "uppercase extension", icon: "icons/plugin.PNG", body: []byte("\x89PNG\r\n\x1a\n")},
		{name: "jpeg", icon: "icons/plugin.jpeg", body: []byte("\xff\xd8\xff")},
		{name: "jpg", icon: "icons/plugin.jpg", body: []byte("\xff\xd8\xff")},
		{name: "webp", icon: "icons/plugin.webp", body: []byte("RIFF")},
		{name: "gif", icon: "icons/plugin.gif", body: []byte("GIF89a")},
		{name: "nested path", icon: "assets/brand/plugin.png", body: []byte("png")},
		{name: "size limit", icon: "icons/plugin.png", body: make([]byte, maxArtifactIconBytes)},
	}
	for _, format := range []string{".zip", ".tgz"} {
		for _, test := range cases {
			t.Run(format+"/"+test.name, func(t *testing.T) {
				manifest := artifactTestChange(t, func(fields map[string]any) { fields["icon"] = test.icon })
				entries := append(artifactTestEntries(manifest), artifactTestEntry{name: test.icon, body: test.body})
				filename := artifactTestWrite(t, format, artifactTestArchive(t, format, entries))
				if _, err := ValidatePackage(context.Background(), filename); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestArtifactPluginIconRejectsInvalidDeclarations(t *testing.T) {
	cases := []struct {
		name    string
		icon    string
		present bool
		body    []byte
	}{
		{name: "svg", icon: "icons/plugin.svg", present: true, body: []byte("<svg/>")},
		{name: "absolute", icon: "/icons/plugin.png", present: true, body: []byte("png")},
		{name: "traversal", icon: "../plugin.png", present: true, body: []byte("png")},
		{name: "backslash", icon: `icons\plugin.png`, present: true, body: []byte("png")},
		{name: "long path", icon: strings.Repeat("a", 253) + ".png", present: true, body: []byte("png")},
		{name: "missing file", icon: "icons/plugin.png"},
		{name: "implicit directory", icon: "bin"},
		{name: "oversized", icon: "icons/plugin.png", present: true, body: make([]byte, maxArtifactIconBytes+1)},
	}
	for _, format := range []string{".zip", ".tgz"} {
		for _, test := range cases {
			t.Run(format+"/"+test.name, func(t *testing.T) {
				manifest := artifactTestChange(t, func(fields map[string]any) { fields["icon"] = test.icon })
				entries := artifactTestEntries(manifest)
				if test.present {
					entries = append(entries, artifactTestEntry{name: test.icon, body: test.body})
				}
				filename := artifactTestWrite(t, format, artifactTestArchive(t, format, entries))
				if _, err := ValidatePackage(context.Background(), filename); err == nil {
					t.Fatal("accepted invalid plugin icon declaration")
				}
			})
		}
	}
}

func TestArtifactAIProviderIconRespectsSizeLimit(t *testing.T) {
	manifest := artifactAIProviderManifest(t, func(_ map[string]any, provider map[string]any, _ map[string]any) {
		provider["icon"] = "icons/provider.png"
	})
	accepted := append(artifactTestEntries(manifest), artifactTestEntry{name: "icons/provider.png", body: make([]byte, maxArtifactIconBytes)})
	filename := artifactTestWrite(t, ".zip", artifactTestZIP(t, accepted))
	if _, err := ValidatePackage(context.Background(), filename); err != nil {
		t.Fatalf("icon at the size limit rejected: %v", err)
	}
	rejected := append(artifactTestEntries(manifest), artifactTestEntry{name: "icons/provider.png", body: make([]byte, maxArtifactIconBytes+1)})
	filename = artifactTestWrite(t, ".zip", artifactTestZIP(t, rejected))
	if _, err := ValidatePackage(context.Background(), filename); err == nil {
		t.Fatal("oversized ai provider icon accepted")
	}
}
