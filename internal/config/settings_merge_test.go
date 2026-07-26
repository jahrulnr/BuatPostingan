package config

import (
	"testing"

	"buatpostingan/internal/domain/entity"
)

func TestMaskAPIKey(t *testing.T) {
	set, masked := MaskAPIKey("")
	if set || masked != "" {
		t.Fatalf("empty: set=%v masked=%q", set, masked)
	}
	set, masked = MaskAPIKey("abcd")
	if !set || masked != "••••" {
		t.Fatalf("short: %v %q", set, masked)
	}
	set, masked = MaskAPIKey("sk-or-v1-abcdefgh")
	if !set || masked != "••••…efgh" {
		t.Fatalf("long: %v %q", set, masked)
	}
}

func TestApplySettingsFileEmptyKeepsEnv(t *testing.T) {
	base := Config{
		LLMProviders: map[string]LLMProvider{
			"OPENROUTER": {ID: "OPENROUTER", APIKey: "env-key", Enabled: true},
		},
		LLMStub: false,
	}
	got := ApplySettingsFile(base, entity.SettingsFile{})
	if got.LLMProviders["OPENROUTER"].APIKey != "env-key" {
		t.Fatal("should keep env providers when file empty")
	}
}

func TestApplySettingsFileReplacesProviders(t *testing.T) {
	base := Config{
		LLMProviders: map[string]LLMProvider{
			"OPENROUTER": {ID: "OPENROUTER", APIKey: "env-key", Enabled: true},
		},
		LLMStrategy: "failover",
		LLMStub:     false,
	}
	doc := entity.SettingsFile{
		LLM: entity.SettingsLLM{
			Strategy:       "switch",
			ActiveProvider: "LOCAL",
			Providers: []entity.SettingsProvider{
				{
					ID:      "LOCAL",
					Name:    "Local",
					API:     "chat",
					BaseURL: "http://127.0.0.1/v1",
					APIKey:  "file-key",
					Enabled: true,
					Models: []entity.SettingsModel{
						{ID: "mimo", OutputModes: []string{"text"}},
						{ID: "image-only", Task: "image-generation", OutputModes: []string{"image"}},
					},
				},
			},
		},
	}
	got := ApplySettingsFile(base, doc)
	if _, ok := got.LLMProviders["OPENROUTER"]; ok {
		t.Fatal("env provider should be replaced")
	}
	p := got.LLMProviders["LOCAL"]
	if p.APIKey != "file-key" || p.Model != "mimo" || p.API != "chat" {
		t.Fatalf("provider: %+v", p)
	}
	if got.LLMStrategy != "switch" || got.LLMActiveProvider != "LOCAL" {
		t.Fatalf("strategy/active: %s %s", got.LLMStrategy, got.LLMActiveProvider)
	}
	if got.LLMStub {
		t.Fatal("stub should be false (key present)")
	}
	models := got.LLMModels["LOCAL"]
	if len(models) != 2 || len(models[0].OutputModes) != 1 || models[0].OutputModes[0] != "text" {
		t.Fatalf("runtime model metadata: %+v", models)
	}
	if models[1].Task != "image-generation" {
		t.Fatalf("runtime model task: %+v", models[1])
	}
}

func TestApplySettingsFileMCPWithoutLLMProviders(t *testing.T) {
	base := Config{
		MCPEnabled:           true,
		MCPConnectTimeoutSec: 15,
		MCPCallTimeoutSec:    30,
		LLMProviders: map[string]LLMProvider{
			"OPENROUTER": {ID: "OPENROUTER", APIKey: "env-key", Enabled: true},
		},
	}
	en := false
	doc := entity.SettingsFile{
		MCP: entity.SettingsMCP{
			Enabled:           &en,
			ConnectTimeoutSec: 9,
			CallTimeoutSec:    11,
			Servers: []entity.SettingsMCPServer{{
				ID:         "echo",
				Transport:  "stdio",
				Command:    "./bin/mcp-echo",
				Enabled:    true,
				Trusted:    true,
				AllowTools: []string{"echo"},
			}},
		},
	}
	got := ApplySettingsFile(base, doc)
	if got.MCPEnabled {
		t.Fatal("mcp.enabled=false should apply")
	}
	if got.MCPConnectTimeoutSec != 9 || got.MCPCallTimeoutSec != 11 {
		t.Fatalf("timeouts: %d %d", got.MCPConnectTimeoutSec, got.MCPCallTimeoutSec)
	}
	if len(got.MCPServers) != 1 || got.MCPServers[0].ID != "echo" || !got.MCPServers[0].Trusted {
		t.Fatalf("servers: %+v", got.MCPServers)
	}
	// LLM providers unchanged when file llm.providers empty
	if got.LLMProviders["OPENROUTER"].APIKey != "env-key" {
		t.Fatal("env LLM providers should remain")
	}
}

func TestDefaultLocalDevMCP(t *testing.T) {
	m := DefaultLocalDevMCP()
	if m.Enabled == nil || !*m.Enabled {
		t.Fatal("enabled")
	}
	if len(m.Servers) != 1 || m.Servers[0].ID != "echo" || m.Servers[0].Command != "./bin/mcp-echo" {
		t.Fatalf("%+v", m)
	}
}

func TestDefaultSeedFile(t *testing.T) {
	base := Config{
		LLMStrategy:           "failover",
		LLMStream:             true,
		LLMVision:             "auto",
		LLMEffort:             "auto",
		MaxToolRounds:         8,
		ContextMaxInputTokens: 12000,
		DocsAppID:             "buatpostingan",
	}
	doc := DefaultSeedFile(base)
	if doc.Version != 1 || len(doc.Users) != 1 || doc.Users[0].ID != "usr_owner" {
		t.Fatalf("version/users: %+v", doc)
	}
	if doc.LLM.Strategy != "failover" || doc.LLM.Stream == nil || !*doc.LLM.Stream {
		t.Fatalf("llm globals not seeded: %+v", doc.LLM)
	}
	if doc.Limits.MaxToolRounds == nil || *doc.Limits.MaxToolRounds != 8 {
		t.Fatalf("limits not seeded: %+v", doc.Limits)
	}
	if doc.Context.MaxInputTokens == nil || *doc.Context.MaxInputTokens != 12000 {
		t.Fatalf("context not seeded: %+v", doc.Context)
	}
	if doc.Docs.AppID != "buatpostingan" {
		t.Fatalf("docs.app_id not seeded: %q", doc.Docs.AppID)
	}
	if len(doc.MCP.Servers) != 1 || doc.MCP.Servers[0].ID != "echo" {
		t.Fatalf("mcp seed missing: %+v", doc.MCP)
	}
	if len(doc.LLM.Providers) != 0 {
		t.Fatalf("providers should default to empty, got %+v", doc.LLM.Providers)
	}
}

func TestApplySettingsFileLimitsContextDocsGlobals(t *testing.T) {
	base := Config{
		MaxToolRounds:            8,
		SpeakFloorTTL:            600,
		LockTTL:                  300,
		TurnJobTimeoutSec:        120,
		LLMStream:                true,
		LLMVision:                "auto",
		LLMEffort:                "auto",
		LLMTotalAttemptBudget:    4,
		LLMRetryBaseDelayMS:      250,
		LLMRetryMaxDelayMS:       5000,
		LLMRetryJitter:           0.2,
		LLMStrategy:              "failover",
		LLMActiveProvider:        "OPENROUTER",
		LLMStub:                  false,
		ContextCompactionEnabled: true,
		ContextMaxInputTokens:    12000,
		ContextReserveTokens:     3000,
		ContextRecentTurns:       4,
		ContextSummaryMaxChars:   12000,
		DocsTopK:                 5,
		DocsMinScore:             0.5,
		DocsFuzzyEnabled:         true,
		DocsAppID:                "buatpostingan",
		GitHubToken:              "",
		SkillsRoot:               "env/skills",
	}
	rounds := 4
	timeout := 240
	streamFalse := false
	effortHigh := "high"
	budget := 6
	baseDelay := 500
	maxDelay := 7000
	jitter := 0.3
	compact := false
	maxIn := 20000
	reserve := 4000
	recent := 6
	summary := 20000
	topK := 8
	minScore := 0.7
	fuzzy := false
	doc := entity.SettingsFile{
		Limits: entity.SettingsLimits{
			MaxToolRounds:     &rounds,
			TurnJobTimeoutSec: &timeout,
		},
		LLM: entity.SettingsLLM{
			Stream:             &streamFalse,
			Effort:             effortHigh,
			TotalAttemptBudget: &budget,
			RetryBaseDelayMS:   &baseDelay,
			RetryMaxDelayMS:    &maxDelay,
			RetryJitter:        &jitter,
		},
		Context: entity.SettingsContext{
			CompactionEnabled: &compact,
			MaxInputTokens:    &maxIn,
			ReserveTokens:     &reserve,
			RecentTurns:       &recent,
			SummaryMaxChars:   &summary,
		},
		Docs: entity.SettingsDocs{
			TopK:         &topK,
			MinScore:     &minScore,
			FuzzyEnabled: &fuzzy,
			AppID:        "kit",
		},
		WebSearch: entity.SettingsWebSearch{GitHubToken: "ghp_secret"},
	}
	got := ApplySettingsFile(base, doc)
	if got.MaxToolRounds != 4 || got.TurnJobTimeoutSec != 240 {
		t.Fatalf("limits: %+v", got)
	}
	if got.SpeakFloorTTL != 600 || got.LockTTL != 300 {
		t.Fatalf("limits untouched: %+v", got)
	}
	if got.LLMStream || got.LLMEffort != "high" || got.LLMTotalAttemptBudget != 6 {
		t.Fatalf("llm globals: %+v", got)
	}
	if got.LLMRetryBaseDelayMS != 500 || got.LLMRetryMaxDelayMS != 7000 || got.LLMRetryJitter != 0.3 {
		t.Fatalf("retry: %+v", got)
	}
	if got.ContextCompactionEnabled || got.ContextMaxInputTokens != 20000 || got.ContextReserveTokens != 4000 {
		t.Fatalf("context: %+v", got)
	}
	if got.ContextRecentTurns != 6 || got.ContextSummaryMaxChars != 20000 {
		t.Fatalf("context: %+v", got)
	}
	if got.DocsTopK != 8 || got.DocsMinScore != 0.7 || got.DocsFuzzyEnabled || got.DocsAppID != "kit" {
		t.Fatalf("docs: %+v", got)
	}
	if got.GitHubToken != "ghp_secret" {
		t.Fatalf("github token: %q", got.GitHubToken)
	}
}

func TestApplySettingsFileOmitKeepsEnvDefaults(t *testing.T) {
	base := Config{
		MaxToolRounds: 8,
		LLMStream:     true,
		LLMEffort:     "auto",
		DocsAppID:     "buatpostingan",
		GitHubToken:   "env-gh",
		SkillsRoot:    "env/skills",
		LLMStrategy:   "failover",
		LLMStub:       false,
		LLMProviders:  map[string]LLMProvider{"OPENROUTER": {ID: "OPENROUTER", APIKey: "k", Enabled: true}},
	}
	// Empty doc (missing/old file) — env bootstrap must win.
	got := ApplySettingsFile(base, entity.SettingsFile{})
	if got.MaxToolRounds != 8 || !got.LLMStream || got.LLMEffort != "auto" {
		t.Fatalf("env defaults lost: %+v", got)
	}
	if got.DocsAppID != "buatpostingan" || got.GitHubToken != "env-gh" || got.SkillsRoot != "env/skills" {
		t.Fatalf("env defaults lost: %+v", got)
	}
	if got.LLMProviders["OPENROUTER"].APIKey != "k" {
		t.Fatal("env provider must remain when file providers empty")
	}
}
