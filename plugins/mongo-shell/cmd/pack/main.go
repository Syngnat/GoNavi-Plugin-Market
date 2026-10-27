// Command pack builds the MongoShell plugin package for one or more platforms.
//
// It mirrors the market's cmd/buildtool: every platform binary is compiled with
// CGO disabled, ui/skills/README/LICENSE are copied into a fresh package
// directory and the manifest is written last. The one deliberate difference is
// that the manifest is edited as raw JSON instead of being round-tripped through
// the published Go SDK struct: the SDK release this module pins predates
// contributes.connectionActions, so buildtool would silently drop the window
// entry point of this plugin. Once the SDK publishes that field this tool can be
// deleted in favour of cmd/buildtool.
package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type target struct {
	goos   string
	goarch string
}

func (t target) key() string { return t.goos + "/" + t.goarch }

func (t target) executable() string {
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
	manifest  string
	output    string
	zipPath   string
	platforms string
}

func main() {
	var opts options
	flag.StringVar(&opts.manifest, "manifest", "gonavi.plugin.json", "manifest template")
	flag.StringVar(&opts.output, "output", "", "new package directory (must not exist)")
	flag.StringVar(&opts.zipPath, "zip", "", "optional archive path written after the package directory")
	flag.StringVar(&opts.platforms, "platforms", "", "comma-separated GOOS/GOARCH list (default: all five)")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err := run(ctx, opts); err != nil {
		fmt.Fprintln(os.Stderr, "pack:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, opts options) error {
	if strings.TrimSpace(opts.output) == "" {
		return errors.New("-output is required")
	}
	manifestPath, err := filepath.Abs(opts.manifest)
	if err != nil {
		return err
	}
	output, err := filepath.Abs(opts.output)
	if err != nil {
		return err
	}
	if _, err := os.Stat(output); err == nil {
		return fmt.Errorf("output %s already exists", output)
	}
	sourceDir := filepath.Dir(manifestPath)
	targets, err := parseTargets(opts.platforms)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(output, 0o755); err != nil {
		return err
	}
	for _, item := range targets {
		destination := filepath.Join(output, filepath.FromSlash(item.executable()))
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}
		if err := build(ctx, sourceDir, item, destination); err != nil {
			return err
		}
		fmt.Println("built", item.key(), "->", item.executable())
	}
	if err := copyAssets(sourceDir, output); err != nil {
		return err
	}
	manifest, err := writeManifest(manifestPath, output, targets)
	if err != nil {
		return err
	}
	fmt.Println("package written to", output, "with", manifest, "bytes of manifest")
	if opts.zipPath != "" {
		return writeZip(output, opts.zipPath)
	}
	return nil
}

func parseTargets(value string) ([]target, error) {
	if strings.TrimSpace(value) == "" {
		return defaultTargets, nil
	}
	targets := make([]target, 0, 4)
	for _, part := range strings.Split(value, ",") {
		parts := strings.SplitN(strings.TrimSpace(part), "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid platform %q", part)
		}
		targets = append(targets, target{goos: parts[0], goarch: parts[1]})
	}
	if len(targets) == 0 {
		return nil, errors.New("no platforms selected")
	}
	return targets, nil
}

func build(ctx context.Context, sourceDir string, item target, destination string) error {
	command := exec.CommandContext(ctx, "go", "build", "-trimpath", "-ldflags", "-s -w", "-o", destination, ".")
	command.Dir = sourceDir
	command.Env = append(os.Environ(),
		"GOWORK=off", "CGO_ENABLED=0", "GOOS="+item.goos, "GOARCH="+item.goarch)
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("build %s: %w", item.key(), err)
	}
	return nil
}

// writeManifest replaces only the backend entry and preserves every other
// declaration, including contributes.connectionActions.
func writeManifest(manifestPath, output string, targets []target) (int, error) {
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return 0, fmt.Errorf("read manifest template: %w", err)
	}
	encoded, err := applyBackend(raw, targets)
	if err != nil {
		return 0, err
	}
	if err := os.WriteFile(filepath.Join(output, "gonavi.plugin.json"), encoded, 0o644); err != nil {
		return 0, err
	}
	return len(encoded), nil
}

func applyBackend(raw []byte, targets []target) ([]byte, error) {
	var manifest map[string]json.RawMessage
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, fmt.Errorf("decode manifest template: %w", err)
	}
	if manifest == nil {
		return nil, errors.New("manifest template must be a JSON object")
	}
	for _, key := range []string{"schemaVersion", "apiVersion", "id", "name", "version"} {
		if value, ok := manifest[key]; ok && len(value) > 0 {
			continue
		}
		return nil, fmt.Errorf("manifest template is missing %s", key)
	}
	platforms := make(map[string]manifestEntry, len(targets))
	for _, item := range targets {
		platforms[item.key()] = manifestEntry{Executable: item.executable(), Runtime: "native"}
	}
	backend, err := json.Marshal(manifestBackend{Platforms: platforms})
	if err != nil {
		return nil, err
	}
	manifest["backend"] = backend
	return json.MarshalIndent(manifest, "", "  ")
}

type manifestEntry struct {
	Executable string `json:"executable"`
	Runtime    string `json:"runtime"`
}

type manifestBackend struct {
	Platforms map[string]manifestEntry `json:"platforms"`
}

func copyAssets(sourceDir, output string) error {
	for _, directory := range []string{"ui", "skills"} {
		if err := copyDir(filepath.Join(sourceDir, directory), filepath.Join(output, directory)); err != nil {
			return fmt.Errorf("copy %s: %w", directory, err)
		}
	}
	for _, name := range []string{"README.md", "LICENSE"} {
		if err := copyFile(filepath.Join(sourceDir, name), filepath.Join(output, name)); err != nil {
			return fmt.Errorf("copy %s: %w", name, err)
		}
	}
	return nil
}

func copyDir(source, destination string) error {
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		sourcePath := filepath.Join(source, name)
		destinationPath := filepath.Join(destination, name)
		info, err := os.Stat(sourcePath)
		if err != nil {
			return err
		}
		if info.IsDir() {
			if err := copyDir(sourcePath, destinationPath); err != nil {
				return err
			}
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if err := copyFile(sourcePath, destinationPath); err != nil {
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
	return os.WriteFile(destination, data, 0o644)
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
		relative, err := filepath.Rel(packageDir, path)
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relative)
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
