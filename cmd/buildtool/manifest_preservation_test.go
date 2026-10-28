package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestPackagePreservesManifestContributions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake compiler uses a POSIX shell")
	}
	root := t.TempDir()
	compiler := filepath.Join(root, "compiler")
	if err := os.Mkdir(compiler, 0o700); err != nil {
		t.Fatal(err)
	}
	fake := "#!/bin/sh\nwhile [ $# -gt 0 ]; do\n if [ \"$1\" = '-o' ]; then shift; printf 'fixture executable' > \"$1\"; exit 0; fi\n shift\ndone\nexit 1\n"
	if err := os.WriteFile(filepath.Join(compiler, "go"), []byte(fake), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", compiler+string(os.PathListSeparator)+os.Getenv("PATH"))
	manifest := filepath.Join("..", "..", "plugins", "mongo-shell", "gonavi.plugin.json")
	archivePath := filepath.Join(root, "package.zip")
	if err := run(context.Background(), options{manifest: manifest, output: filepath.Join(root, "package"), zipPath: archivePath}); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	for _, entry := range archive.File {
		if entry.Name != "gonavi.plugin.json" {
			continue
		}
		stream, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		packaged, readErr := io.ReadAll(stream)
		closeErr := stream.Close()
		if readErr != nil || closeErr != nil {
			t.Fatalf("read=%v close=%v", readErr, closeErr)
		}
		var want, got map[string]any
		if err := json.Unmarshal(original, &want); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(packaged, &got); err != nil {
			t.Fatal(err)
		}
		backend := got["backend"].(map[string]any)
		if len(backend["platforms"].(map[string]any)) != len(defaultTargets) {
			t.Fatal("missing backend platforms")
		}
		delete(want, "backend")
		delete(got, "backend")
		if !reflect.DeepEqual(want, got) {
			t.Fatal("packaging changed non-backend manifest declarations")
		}
		return
	}
	t.Fatal("archive is missing the manifest")
}
