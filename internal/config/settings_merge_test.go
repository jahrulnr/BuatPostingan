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
	stubFalse := false
	doc := entity.SettingsFile{
		LLM: entity.SettingsLLM{
			Strategy:       "switch",
			ActiveProvider: "LOCAL",
			Stub:           &stubFalse,
			Providers: []entity.SettingsProvider{
				{
					ID:      "LOCAL",
					Name:    "Local",
					API:     "chat",
					BaseURL: "http://127.0.0.1/v1",
					APIKey:  "file-key",
					Enabled: true,
					Models:  []entity.SettingsModel{{ID: "mimo"}},
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
		t.Fatal("stub should be false")
	}
}
