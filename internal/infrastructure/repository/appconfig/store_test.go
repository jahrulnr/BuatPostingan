package appconfig

import (
	"os"
	"path/filepath"
	"testing"

	"buatpostingan/internal/config"
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

func TestEnsureSeededCreatesFileFromDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	s := NewStore(path)

	if s.Exists() {
		t.Fatal("expected missing before seed")
	}
	base := config.Config{
		LLMStrategy:  "failover",
		DocsAppID:    "buatpostingan",
		MaxToolRounds: 8,
	}
	_, created, err := s.EnsureSeeded(nil, base)
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("expected created=true on first seed")
	}
	if !s.Exists() {
		t.Fatal("file should exist after seed")
	}

	doc, err := s.Load(nil)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Version != 1 || len(doc.Users) != 1 || doc.Users[0].Role != "owner" {
		t.Fatalf("seed doc malformed: %+v", doc)
	}
	if doc.Limits.MaxToolRounds == nil || *doc.Limits.MaxToolRounds != 8 {
		t.Fatalf("limits not seeded from base: %+v", doc.Limits)
	}
	if len(doc.MCP.Servers) != 1 || doc.MCP.Servers[0].ID != "echo" {
		t.Fatalf("mcp echo sample missing: %+v", doc.MCP)
	}
	if len(doc.LLM.Providers) != 0 {
		t.Fatalf("providers should be empty by default, got %+v", doc.LLM.Providers)
	}
}

func TestEnsureSeededIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	s := NewStore(path)

	base := config.Config{DocsAppID: "first"}
	if _, _, err := s.EnsureSeeded(nil, base); err != nil {
		t.Fatal(err)
	}
	// Second call with different base must NOT overwrite.
	base2 := config.Config{DocsAppID: "changed"}
	_, created, err := s.EnsureSeeded(nil, base2)
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("expected created=false on second seed")
	}
	doc, _ := s.Load(nil)
	if doc.Docs.AppID != "first" {
		t.Fatalf("expected original seed preserved, got app_id=%q", doc.Docs.AppID)
	}
}

func TestSaveLoadNewSections(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	s := NewStore(path)

	rounds := 4
	streamOff := false
	compact := false
	topK := 8
	minScore := 0.7
	doc := entity.SettingsFile{
		Version: 1,
		Limits:  entity.SettingsLimits{MaxToolRounds: &rounds},
		LLM: entity.SettingsLLM{
			Stream: &streamOff,
			Providers: []entity.SettingsProvider{
				{ID: "LOCAL", API: "responses", BaseURL: "http://x/v1", Enabled: true, Models: []entity.SettingsModel{{ID: "m"}}},
			},
		},
		Context:   entity.SettingsContext{CompactionEnabled: &compact, MaxInputTokens: &[]int{20000}[0]},
		Docs:      entity.SettingsDocs{TopK: &topK, MinScore: &minScore, AppID: "kit"},
		WebSearch: entity.SettingsWebSearch{GitHubToken: "ghp_x"},
	}
	if err := s.Save(nil, doc); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Limits.MaxToolRounds == nil || *got.Limits.MaxToolRounds != 4 {
		t.Fatalf("limits: %+v", got.Limits)
	}
	if got.LLM.Stream == nil || *got.LLM.Stream {
		t.Fatalf("llm.stream: %+v", got.LLM.Stream)
	}
	if got.Context.CompactionEnabled == nil || *got.Context.CompactionEnabled {
		t.Fatalf("context.compaction: %+v", got.Context.CompactionEnabled)
	}
	if got.Context.MaxInputTokens == nil || *got.Context.MaxInputTokens != 20000 {
		t.Fatalf("context.max_input: %+v", got.Context.MaxInputTokens)
	}
	if got.Docs.TopK == nil || *got.Docs.TopK != 8 || got.Docs.MinScore == nil || *got.Docs.MinScore != 0.7 {
		t.Fatalf("docs: %+v", got.Docs)
	}
	if got.WebSearch.GitHubToken != "ghp_x" {
		t.Fatalf("web_search: %+v", got.WebSearch)
	}
}
