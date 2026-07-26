package provider_test

import (
	"testing"

	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/infrastructure/provider"
	ninerouter "buatpostingan/internal/infrastructure/provider/9router"
	"buatpostingan/internal/infrastructure/provider/claude"
	"buatpostingan/internal/infrastructure/provider/omniroute"
	"buatpostingan/internal/infrastructure/provider/openai"
	openaicompatible "buatpostingan/internal/infrastructure/provider/openai-compatible"
	"buatpostingan/internal/infrastructure/provider/openrouter"
)

func newRegistry(t *testing.T) *provider.Registry {
	t.Helper()
	reg, err := provider.NewRegistry(
		openrouter.New(),
		omniroute.New(),
		ninerouter.New(),
		openai.New(),
		openaicompatible.New(),
		claude.New(),
	)
	if err != nil {
		t.Fatal(err)
	}
	return reg
}

func TestRegistryListsStableProviderOrder(t *testing.T) {
	reg := newRegistry(t)
	got := reg.List()
	want := []string{
		"openrouter",
		"omniroute",
		"9router",
		"openai",
		"openai-compatible",
		"claude",
	}
	if len(got) != len(want) {
		t.Fatalf("definitions=%d want=%d", len(got), len(want))
	}
	for i := range want {
		if got[i].Type != want[i] {
			t.Fatalf("definition[%d]=%q want=%q", i, got[i].Type, want[i])
		}
	}
}

func TestRegistryAppliesProviderDefaults(t *testing.T) {
	reg := newRegistry(t)
	p, err := reg.Normalize(entity.SettingsProvider{Type: "omniroute"})
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "OMNIROUTE" || p.BaseURL != "http://127.0.0.1:20128/v1" {
		t.Fatalf("omniroute defaults: %+v", p)
	}
	if !p.APIKeyOptional || p.API != "responses" {
		t.Fatalf("omniroute capabilities: %+v", p)
	}

	nine, err := reg.Normalize(entity.SettingsProvider{Type: "9router"})
	if err != nil {
		t.Fatal(err)
	}
	if nine.ID != "9ROUTER" || nine.BaseURL != "http://127.0.0.1:20128/v1" || nine.API != "chat" {
		t.Fatalf("9router defaults: %+v", nine)
	}
}

func TestRegistryInfersLegacyProvider(t *testing.T) {
	reg := newRegistry(t)
	got := reg.Infer(entity.SettingsProvider{
		ID:      "OPENROUTER",
		BaseURL: "https://openrouter.ai/api/v1",
	})
	if got != "openrouter" {
		t.Fatalf("infer=%q", got)
	}
	got = reg.Infer(entity.SettingsProvider{
		ID:      "MY_PROXY",
		BaseURL: "http://llm.internal/v1",
	})
	if got != "openai-compatible" {
		t.Fatalf("fallback infer=%q", got)
	}
	claudeAPI, err := reg.Normalize(entity.SettingsProvider{Type: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	if claudeAPI.API != "messages" || claudeAPI.BaseURL != "https://api.anthropic.com/v1" {
		t.Fatalf("claude defaults: %+v", claudeAPI)
	}
}
