package appconfig

import (
	"os"
	"path/filepath"
	"testing"

	"buatpostingan/internal/domain/entity"
)

func TestResolvePath(t *testing.T) {
	got := ResolvePath("storage/webchat", "")
	want := filepath.Clean("storage/config.json")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	got = ResolvePath("storage/webchat", "/abs/cfg.json")
	if got != "/abs/cfg.json" {
		t.Fatalf("override: got %q", got)
	}
}

func TestSaveLoadAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	s := NewStore(path)

	doc := entity.SettingsFile{
		Version: 1,
		Users: []entity.SettingsUser{
			{ID: "usr_1", Name: "Owner", Role: "owner"},
		},
		LLM: entity.SettingsLLM{
			Strategy: "failover",
			Providers: []entity.SettingsProvider{
				{
					ID:      "LOCAL",
					Name:    "Local",
					API:     "responses",
					BaseURL: "http://127.0.0.1:20128/v1",
					APIKey:  "sk-secret-key",
					Enabled: true,
					Models:  []entity.SettingsModel{{ID: "mimo"}},
				},
			},
		},
	}
	if err := s.Save(nil, doc); err != nil {
		t.Fatal(err)
	}
	if !s.Exists() {
		t.Fatal("expected file")
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm()&0o077 != 0 {
		t.Fatalf("expected 0600-ish perms, got %v", st.Mode().Perm())
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("tmp should be gone after rename")
	}

	got, err := s.Load(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Users) != 1 || got.Users[0].Name != "Owner" {
		t.Fatalf("users: %+v", got.Users)
	}
	if len(got.LLM.Providers) != 1 || got.LLM.Providers[0].APIKey != "sk-secret-key" {
		t.Fatalf("providers: %+v", got.LLM.Providers)
	}
}

func TestLoadMissing(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "nope.json"))
	_, err := s.Load(nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
