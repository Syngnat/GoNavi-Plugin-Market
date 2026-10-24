// Package store persists SSH connection profiles outside the immutable
// plugin package. Secrets stay on disk with restrictive permissions and are
// never returned to the UI in full.
package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// AuthType selects the SSH authentication material.
const (
	AuthPassword = "password"
	AuthKey      = "key"
)

// Profile is one stored SSH connection. Password fields are persisted but
// masked by MaskProfiles before anything leaves the process.
type Profile struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
	Username      string `json:"username"`
	AuthType      string `json:"authType"`
	Password      string `json:"password,omitempty"`
	KeyPath       string `json:"keyPath,omitempty"`
	KeyPassphrase string `json:"keyPassphrase,omitempty"`
	Fingerprint   string `json:"fingerprint,omitempty"`
	CreatedAt     string `json:"createdAt,omitempty"`
	UpdatedAt     string `json:"updatedAt,omitempty"`
}

// MaskedProfile is the wire form; secrets are replaced by presence flags.
type MaskedProfile struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	AuthType    string `json:"authType"`
	HasPassword bool   `json:"hasPassword"`
	KeyPath     string `json:"keyPath,omitempty"`
	HasKeyPass  bool   `json:"hasKeyPassphrase"`
	Fingerprint string `json:"fingerprint,omitempty"`
	CreatedAt   string `json:"createdAt,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
}

type file struct {
	Profiles []Profile `json:"profiles"`
}

// Store serializes access to the profiles file.
type Store struct {
	path string
	mu   sync.Mutex
}

// DefaultPath returns the profile file location. Inside the host runtime the
// GONAVI_PLUGIN_DATA environment variable carries the host-owned private data
// directory; standalone development runs fall back to the user config dir.
func DefaultPath() (string, error) {
	if root := strings.TrimSpace(os.Getenv("GONAVI_PLUGIN_DATA")); root != "" {
		if !filepath.IsAbs(root) || strings.ContainsRune(root, 0) {
			return "", fmt.Errorf("GONAVI_PLUGIN_DATA must be an absolute path")
		}
		return filepath.Join(root, "connections.json"), nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(base, "GoNavi", "plugin-data", "com.gonavi.nginx-inspector", "connections.json"), nil
}

// Open returns a Store backed by path; the file is created on first write.
func Open(path string) *Store { return &Store{path: path} }

func (s *Store) load() (file, error) {
	var data file
	raw, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return data, nil
	}
	if err != nil {
		return data, fmt.Errorf("read profiles: %w", err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return file{}, fmt.Errorf("decode profiles: %w", err)
	}
	return data, nil
}

func (s *Store) persist(data file) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create profile directory: %w", err)
	}
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, encoded, 0o600); err != nil {
		return fmt.Errorf("write profiles: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("commit profiles: %w", err)
	}
	return nil
}

// List returns all profiles in stable order.
func (s *Store) List() ([]Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.load()
	if err != nil {
		return nil, err
	}
	sort.Slice(data.Profiles, func(i, j int) bool { return data.Profiles[i].Name < data.Profiles[j].Name })
	return data.Profiles, nil
}

// Get returns one profile by id.
func (s *Store) Get(id string) (Profile, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.load()
	if err != nil {
		return Profile{}, false, err
	}
	for _, p := range data.Profiles {
		if p.ID == id {
			return p, true, nil
		}
	}
	return Profile{}, false, nil
}

// Save validates and upserts a profile, assigning id and timestamps.
func (s *Store) Save(p Profile) (Profile, error) {
	if err := validate(p); err != nil {
		return Profile{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.load()
	if err != nil {
		return Profile{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if p.ID == "" {
		id, err := newID()
		if err != nil {
			return Profile{}, err
		}
		p.ID = id
		p.CreatedAt = now
		p.UpdatedAt = now
		data.Profiles = append(data.Profiles, p)
	} else {
		found := false
		for i := range data.Profiles {
			if data.Profiles[i].ID == p.ID {
				p.CreatedAt = data.Profiles[i].CreatedAt
				p.UpdatedAt = now
				data.Profiles[i] = p
				found = true
				break
			}
		}
		if !found {
			return Profile{}, fmt.Errorf("profile %s not found", p.ID)
		}
	}
	if err := s.persist(data); err != nil {
		return Profile{}, err
	}
	return p, nil
}

// Delete removes one profile by id.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.load()
	if err != nil {
		return err
	}
	kept := data.Profiles[:0]
	found := false
	for _, p := range data.Profiles {
		if p.ID == id {
			found = true
			continue
		}
		kept = append(kept, p)
	}
	if !found {
		return fmt.Errorf("profile %s not found", id)
	}
	data.Profiles = kept
	return s.persist(data)
}

// SetFingerprint records the first-seen host key fingerprint (TOFU).
func (s *Store) SetFingerprint(id, fingerprint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.load()
	if err != nil {
		return err
	}
	for i := range data.Profiles {
		if data.Profiles[i].ID == id {
			if data.Profiles[i].Fingerprint == "" {
				data.Profiles[i].Fingerprint = fingerprint
				data.Profiles[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
				return s.persist(data)
			}
			return nil
		}
	}
	return fmt.Errorf("profile %s not found", id)
}

// MaskProfiles strips secrets for the wire.
func MaskProfiles(in []Profile) []MaskedProfile {
	out := make([]MaskedProfile, 0, len(in))
	for _, p := range in {
		out = append(out, MaskedProfile{
			ID: p.ID, Name: p.Name, Host: p.Host, Port: p.Port,
			Username: p.Username, AuthType: p.AuthType,
			HasPassword: p.Password != "", KeyPath: p.KeyPath,
			HasKeyPass: p.KeyPassphrase != "", Fingerprint: p.Fingerprint,
			CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
		})
	}
	return out
}

func validate(p Profile) error {
	if strings.TrimSpace(p.Name) == "" || len(p.Name) > 100 {
		return fmt.Errorf("profile name must be 1-100 characters")
	}
	if strings.TrimSpace(p.Host) == "" || len(p.Host) > 255 || strings.ContainsAny(p.Host, " \t\r\n\x00") {
		return fmt.Errorf("profile host is missing or invalid")
	}
	if p.Port < 1 || p.Port > 65535 {
		return fmt.Errorf("profile port must be 1-65535")
	}
	if strings.TrimSpace(p.Username) == "" || len(p.Username) > 100 {
		return fmt.Errorf("profile username must be 1-100 characters")
	}
	if len(p.Password) > 1024 || len(p.KeyPath) > 1024 || len(p.KeyPassphrase) > 1024 {
		return fmt.Errorf("credential field exceeds 1024 bytes")
	}
	switch p.AuthType {
	case AuthPassword:
		if p.Password == "" {
			return fmt.Errorf("password is required for password auth")
		}
	case AuthKey:
		if strings.TrimSpace(p.KeyPath) == "" {
			return fmt.Errorf("key path is required for key auth")
		}
	default:
		return fmt.Errorf("authType must be password or key")
	}
	return nil
}

func newID() (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate profile id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
