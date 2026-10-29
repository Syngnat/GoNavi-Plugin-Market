package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeCompiler replaces the go tool with a POSIX shell stub so packaging tests stay hermetic.
func fakeCompiler(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake compiler uses a POSIX shell")
	}
	directory := filepath.Join(t.TempDir(), "compiler")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	fake := "#!/bin/sh\nwhile [ $# -gt 0 ]; do\n if [ \"$1\" = '-o' ]; then shift; printf 'fixture executable' > \"$1\"; exit 0; fi\n shift\ndone\nexit 1\n"
	if err := os.WriteFile(filepath.Join(directory, "go"), []byte(fake), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func packagePlugin(t *testing.T, manifestPath string) (*zip.ReadCloser, string) {
	t.Helper()
	root := t.TempDir()
	archivePath := filepath.Join(root, "package.zip")
	if err := run(context.Background(), options{
		manifest: manifestPath,
		output:   filepath.Join(root, "package"),
		zipPath:  archivePath,
	}); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = archive.Close() })
	return archive, filepath.Join(root, "package")
}

func readArchiveEntry(t *testing.T, archive *zip.ReadCloser, name string) []byte {
	t.Helper()
	for _, entry := range archive.File {
		if entry.Name != name {
			continue
		}
		stream, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, readErr := io.ReadAll(stream)
		closeErr := stream.Close()
		if readErr != nil || closeErr != nil {
			t.Fatalf("read=%v close=%v", readErr, closeErr)
		}
		return content
	}
	return nil
}

// TestPackageCopiesDeclaredIcon pins the icon contract: whatever the manifest names under
// contributes.aiProviders[].icon must exist in the package with identical bytes.
func TestPackageCopiesDeclaredIcon(t *testing.T) {
	fakeCompiler(t)
	source := filepath.Join("..", "..", "plugins", "hualong-ai-provider")
	archive, packageDir := packagePlugin(t, filepath.Join(source, "gonavi.plugin.json"))

	want, err := os.ReadFile(filepath.Join(source, "icons", "provider.png"))
	if err != nil {
		t.Fatal(err)
	}
	got := readArchiveEntry(t, archive, "icons/provider.png")
	if len(got) == 0 {
		t.Fatal("the archive is missing the declared icon")
	}
	if !bytes.Equal(got, want) {
		t.Fatal("the packaged icon does not match its source bytes")
	}
	packaged, err := os.ReadFile(filepath.Join(packageDir, "icons", "provider.png"))
	if err != nil || !bytes.Equal(packaged, want) {
		t.Fatalf("the package directory icon is wrong: %v", err)
	}

	manifest := readArchiveEntry(t, archive, "gonavi.plugin.json")
	var decoded struct {
		Contributes struct {
			AIProviders []struct {
				Icon   string `json:"icon"`
				Models []any  `json:"models"`
			} `json:"aiProviders"`
		} `json:"contributes"`
	}
	if err := json.Unmarshal(manifest, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Contributes.AIProviders) != 1 {
		t.Fatal("the packaged manifest lost its provider")
	}
	if decoded.Contributes.AIProviders[0].Icon != "icons/provider.png" {
		t.Fatalf("unexpected packaged icon path: %q", decoded.Contributes.AIProviders[0].Icon)
	}
}

// TestPackageWithoutIconsDirectory covers the nginx-inspector shape: no icons/ directory.
func TestPackageWithoutIconsDirectory(t *testing.T) {
	fakeCompiler(t)
	source := filepath.Join("..", "..", "plugins", "nginx-inspector")
	if _, err := os.Stat(filepath.Join(source, "icons")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("this fixture must not ship an icons directory")
	}
	archive, packageDir := packagePlugin(t, filepath.Join(source, "gonavi.plugin.json"))
	for _, entry := range archive.File {
		if strings.HasPrefix(entry.Name, "icons/") {
			t.Fatalf("unexpected packaged icon: %s", entry.Name)
		}
	}
	if _, err := os.Stat(filepath.Join(packageDir, "icons")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a plugin without icons must not get an icons directory")
	}
}

// TestCopyOptionalDirRejectsNonDirectory keeps the skip rule narrow: only absence is ignored.
func TestCopyOptionalDirRejectsNonDirectory(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "icons")
	if err := os.WriteFile(source, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := copyOptionalDir(source, filepath.Join(root, "out")); err == nil {
		t.Fatal("a file named icons must stay a hard failure")
	}
	if err := copyOptionalDir(filepath.Join(root, "absent"), filepath.Join(root, "out2")); err != nil {
		t.Fatalf("absent directories must be skipped: %v", err)
	}
}
