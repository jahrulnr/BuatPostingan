// Package appconfig persists product settings as atomic JSON (no DB).
package appconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"buatpostingan/internal/config"
	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/repository"
)

// Store is a JSON file-backed SettingsStore.
type Store struct {
	path string
}

var _ repository.SettingsStore = (*Store)(nil)

// NewStore resolves the config path. Empty path → error at first I/O.
func NewStore(path string) *Store {
	return &Store{path: strings.TrimSpace(path)}
}

func (s *Store) Path() string { return s.path }

func (s *Store) Exists() bool {
	if s.path == "" {
		return false
	}
	st, err := os.Stat(s.path)
	return err == nil && !st.IsDir()
}

// EnsureSeeded writes a default config.json derived from base when the file is
// missing. Returns (path, created, err). When the file already exists it is
// left untouched. Atomic write via Save(); 0600 perms enforced there.
func (s *Store) EnsureSeeded(ctx context.Context, base config.Config) (string, bool, error) {
	if s.path == "" {
		return "", false, fmt.Errorf("config path empty")
	}
	if s.Exists() {
		return s.path, false, nil
	}
	if err := s.Save(ctx, config.DefaultSeedFile(base)); err != nil {
		return s.path, false, err
	}
	return s.path, true, nil
}

func (s *Store) Load(_ context.Context) (entity.SettingsFile, error) {
	var empty entity.SettingsFile
	if s.path == "" {
		return empty, fmt.Errorf("config path empty")
	}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return empty, err
	}
	var doc entity.SettingsFile
	if err := json.Unmarshal(raw, &doc); err != nil {
		return empty, fmt.Errorf("parse config.json: %w", err)
	}
	if doc.Version == 0 {
		doc.Version = 1
	}
	if doc.Users == nil {
		doc.Users = []entity.SettingsUser{}
	}
	if doc.LLM.Providers == nil {
		doc.LLM.Providers = []entity.SettingsProvider{}
	}
	return doc, nil
}

// Save writes atomically (tmp + rename) with 0600 perms.
func (s *Store) Save(_ context.Context, doc entity.SettingsFile) error {
	if s.path == "" {
		return fmt.Errorf("config path empty")
	}
	if doc.Version == 0 {
		doc.Version = 1
	}
	if doc.Users == nil {
		doc.Users = []entity.SettingsUser{}
	}
	if doc.LLM.Providers == nil {
		doc.LLM.Providers = []entity.SettingsProvider{}
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')

	tmp := s.path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, werr := f.Write(raw)
	serr := f.Sync()
	cerr := f.Close()
	if werr != nil {
		_ = os.Remove(tmp)
		return werr
	}
	if serr != nil {
		_ = os.Remove(tmp)
		return serr
	}
	if cerr != nil {
		_ = os.Remove(tmp)
		return cerr
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	_ = os.Chmod(s.path, 0o600)
	return nil
}

// ResolvePath picks BP_CONFIG_PATH or sibling of storage root.
func ResolvePath(storageRoot, override string) string {
	if p := strings.TrimSpace(override); p != "" {
		return p
	}
	root := strings.TrimSpace(storageRoot)
	if root == "" {
		root = "storage/webchat"
	}
	return filepath.Clean(filepath.Join(filepath.Dir(root), "config.json"))
}
