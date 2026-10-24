// Command buildtool compiles the plugin for every advertised platform,
// copies runtime assets into a fresh package directory and writes the final
// manifest with backend.platforms entries. It downloads nothing: module
// dependencies must already be in the local module cache (go mod download)
// or vendored.
package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	plugin "github.com/Syngnat/GoNavi/sdk/go"
)

type target struct{ goos, goarch string }

func (t target) key() string { return t.goos + "/" + t.goarch }

func (t target) entryName() string {
	name := "bin/plugin-" + t.goos + "-" + t.goarch
	if t.goos == "windows" {
		name += ".exe"
	}
	return name
}

var defaultTargets = []target{
	{"windows", "amd64"},
	{"linux", "amd64"},
	{"linux", "arm64"},
	{"darwin", "amd64"},
	{"darwin", "arm64"},
}

type options struct {
	output    string
	manifest  string
	zipPath   string
	platforms string
}

func main() {
	var opts options
	flag.StringVar(&opts.output, "output", "", "new package directory (must not exist)")
	flag.StringVar(&opts.manifest, "manifest", "gonavi.plugin.json", "manifest template")
	flag.StringVar(&opts.zipPath, "zip", "", "optional archive path written after the package directory")
	flag.StringVar(&opts.platforms, "platforms", "", "comma-separated GOOS/GOARCH list (default: all five)")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err := run(ctx, opts); err != nil {
		fmt.Fprintln(os.Stderr, "buildtool:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, opts options) error {
	if opts.output == "" {
		return fmt.Errorf("-output is required")
	}
	output, err := filepath.Abs(opts.output)
	if err != nil {
		return err
	}
	if _, err := os.Stat(output); err == nil {
		return fmt.Errorf("output %s already exists", output)
	}
	manifest, err := buildManifest(ctx, opts, output)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(output, "gonavi.plugin.json"), append(encoded, '\n'), 0o600); err != nil {
		return err
	}
	fmt.Println("package written to", output)
	if opts.zipPath != "" {
		return writeZip(output, opts.zipPath)
	}
	return nil
}

func buildManifest(ctx context.Context, opts options, output string) (*plugin.Manifest, error) {
	raw, err := os.ReadFile(opts.manifest)
	if err != nil {
		return nil, fmt.Errorf("read manifest template: %w", err)
	}
	// Everything plugin-specific resolves against the manifest's directory, so
	// the tool works from any cwd inside the monorepo.
	sourceDir, err := filepath.Abs(filepath.Dir(opts.manifest))
	if err != nil {
		return nil, err
	}
	var manifest plugin.Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, fmt.Errorf("decode manifest template: %w", err)
	}
	targets := defaultTargets
	if opts.platforms != "" {
		targets = nil
		for _, part := range strings.Split(opts.platforms, ",") {
			parts := strings.SplitN(strings.TrimSpace(part), "/", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid platform %q", part)
			}
			targets = append(targets, target{goos: parts[0], goarch: parts[1]})
		}
	}
	if err := os.MkdirAll(output, 0o700); err != nil {
		return nil, err
	}
	platforms := map[string]plugin.Entry{}
	for _, t := range targets {
		entryPath := t.entryName()
		if err := os.MkdirAll(filepath.Join(output, filepath.Dir(entryPath)), 0o700); err != nil {
			return nil, err
		}
		if err := buildOne(ctx, sourceDir, t, filepath.Join(output, filepath.FromSlash(entryPath))); err != nil {
			return nil, err
		}
		platforms[t.key()] = plugin.Entry{Executable: entryPath, Runtime: "native"}
		fmt.Println("built", t.key(), "->", entryPath)
	}
	if len(platforms) == 0 {
		return nil, fmt.Errorf("no platforms selected")
	}
	manifest.Backend = &plugin.Backend{Platforms: platforms}
	for _, dir := range []string{"ui", "skills"} {
		if err := copyDir(filepath.Join(sourceDir, dir), filepath.Join(output, dir)); err != nil {
			return nil, fmt.Errorf("copy %s: %w", dir, err)
		}
	}
	for _, name := range []string{"README.md", "LICENSE"} {
		if err := copyFile(filepath.Join(sourceDir, name), filepath.Join(output, name)); err != nil {
			return nil, fmt.Errorf("copy %s: %w", name, err)
		}
	}
	return &manifest, nil
}

func buildOne(ctx context.Context, sourceDir string, t target, outputPath string) error {
	command := exec.CommandContext(ctx, "go", "build", "-trimpath", "-o", outputPath, ".")
	command.Dir = sourceDir
	command.Env = append(os.Environ(),
		"GOWORK=off", "CGO_ENABLED=0", "GOOS="+t.goos, "GOARCH="+t.goarch)
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("build %s: %w", t.key(), err)
	}
	return nil
}

func copyDir(source, destination string) error {
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(destination, 0o700); err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			if err := copyDir(filepath.Join(source, entry.Name()), filepath.Join(destination, entry.Name())); err != nil {
				return err
			}
			continue
		}
		if !entry.Type().IsRegular() {
			continue
		}
		if err := copyFile(filepath.Join(source, entry.Name()), filepath.Join(destination, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(source, destination string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return os.WriteFile(destination, data, 0o600)
}

func writeZip(packageDir, zipPath string) error {
	archive, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer archive.Close()
	writer := zip.NewWriter(archive)
	defer writer.Close()
	return filepath.Walk(packageDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(packageDir, path)
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		header.Method = zip.Deflate
		entry, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(entry, file)
		return err
	})
}
