package llm

import (
	"context"
	"testing"

	"buatpostingan/internal/config"
)

func TestCatalogStubListAndResolve(t *testing.T) {
	t.Parallel()
	app := config.Config{
		LLMStub:   true,
		LLMEffort: "auto",
		LLMProviders: map[string]config.LLMProvider{
			"OPENROUTER": {ID: "OPENROUTER", Model: "openai/gpt-4o-mini", Enabled: true},
		},
	}
	c := NewCatalog(app, nil, nil)
	cat, err := c.ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !cat.Stub || len(cat.Models) < 2 {
		t.Fatalf("stub catalog: %+v", cat)
	}
	if cat.DefaultModelID == "" {
		t.Fatal("expected default")
	}
	pid, err := c.ResolveModel(context.Background(), "stub/reasoning")
	if err != nil || pid != "STUB" {
		t.Fatalf("resolve stub: %q %v", pid, err)
	}
	if _, err := c.ResolveModel(context.Background(), "evil/model"); err == nil {
		t.Fatal("expected reject")
	}
}

func TestCatalogRealFromProviders(t *testing.T) {
	t.Parallel()
	app := config.Config{
		LLMStub:           false,
		LLMEffort:         "medium",
		LLMActiveProvider: "OPENROUTER",
		LLMProviders: map[string]config.LLMProvider{
			"OPENROUTER": {
				ID: "OPENROUTER", Model: "openai/o3-mini", Enabled: true,
				BaseURL: "http://127.0.0.1:9", APIKey: "sk",
			},
			"LOCAL": {
				ID: "LOCAL", Model: "local-model", Enabled: true, APIKey: "x",
			},
			"OFF": {ID: "OFF", Model: "x", Enabled: false, APIKey: "x"},
		},
	}
	c := NewCatalog(app, NewVisionPolicy(FromApp(app)), NewEffortPolicy(FromApp(app)))
	cat, err := c.ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cat.Stub {
		t.Fatal("expected non-stub")
	}
	if len(cat.Models) != 2 {
		t.Fatalf("models=%d %+v", len(cat.Models), cat.Models)
	}
	if cat.DefaultModelID != "openai/o3-mini" {
		t.Fatalf("default %q", cat.DefaultModelID)
	}
	pid, err := c.ResolveModel(context.Background(), "local-model")
	if err != nil || pid != "LOCAL" {
		t.Fatalf("got %q %v", pid, err)
	}
	pid, err = c.ResolveModel(context.Background(), "OPENROUTER")
	if err != nil || pid != "OPENROUTER" {
		t.Fatalf("provider id: %q %v", pid, err)
	}
	if _, err := c.ResolveModel(context.Background(), "not-configured"); err == nil {
		t.Fatal("expected reject")
	}
}

func TestCatalogOnlySelectsModelsWithTextOutput(t *testing.T) {
	t.Parallel()
	app := config.Config{
		LLMStub:           false,
		LLMActiveProvider: "OPENAI",
		LLMProviders: map[string]config.LLMProvider{
			"OPENAI": {
				ID: "OPENAI", Model: "gpt-text", Enabled: true, APIKey: "sk",
			},
		},
		LLMModelLists: map[string][]string{
			"OPENAI": {"gpt-text", "gpt-image", "legacy-unknown"},
		},
		LLMModels: map[string][]config.LLMModel{
			"OPENAI": {
				{ID: "gpt-text", OutputModes: []string{"text"}},
				{ID: "gpt-image", OutputModes: []string{"image"}},
				{ID: "legacy-unknown"},
			},
		},
	}

	c := NewCatalog(app, nil, nil)
	cat, err := c.ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Models) != 2 {
		t.Fatalf("selectable models: %+v", cat.Models)
	}
	if cat.Models[0].ID != "gpt-text" || cat.Models[1].ID != "legacy-unknown" {
		t.Fatalf("unexpected selectable models: %+v", cat.Models)
	}
	if _, err := c.ResolveModel(context.Background(), "gpt-image"); err == nil {
		t.Fatal("image-only model must not resolve as a chat model")
	}
}

func TestCatalogFiltersKnownNonChatModelsForEveryOpenAIProtocolProvider(t *testing.T) {
	t.Parallel()
	providerTypes := []string{"openai", "openai-compatible", "openrouter", "omniroute", "9router"}
	nonChatModels := []string{
		"text-embedding-3-small",
		"qwen3-embedding-8b",
		"gpt-4o-mini-tts",
		"whisper-1",
		"gpt-4o-transcribe",
		"gpt-image-1",
		"dall-e-3",
		"omni-moderation-latest",
		"bge-reranker-v2-m3",
		"sora-2",
	}

	for _, providerType := range providerTypes {
		providerType := providerType
		t.Run(providerType, func(t *testing.T) {
			t.Parallel()
			modelIDs := append([]string{"gpt-4o", "gpt-realtime", "gpt-audio"}, nonChatModels...)
			models := make([]config.LLMModel, 0, len(modelIDs))
			for _, id := range modelIDs {
				models = append(models, config.LLMModel{ID: id})
			}
			app := config.Config{
				LLMProviders: map[string]config.LLMProvider{
					"PROVIDER": {
						Type: providerType, ID: "PROVIDER", Model: "gpt-4o",
						API: "responses", Enabled: true, APIKey: "test-key",
					},
				},
				LLMModelLists: map[string][]string{"PROVIDER": modelIDs},
				LLMModels:     map[string][]config.LLMModel{"PROVIDER": models},
			}

			catalog := NewCatalog(app, nil, nil)
			got, err := catalog.ListModels(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Models) != 3 {
				t.Fatalf("selectable models for %s: %+v", providerType, got.Models)
			}
			for _, id := range nonChatModels {
				if _, err := catalog.ResolveModel(context.Background(), id); err == nil {
					t.Fatalf("%s must not resolve for %s", id, providerType)
				}
			}
		})
	}
}

func TestCatalogFiltersExplicitNonChatTaskMetadata(t *testing.T) {
	t.Parallel()
	app := config.Config{
		LLMProviders: map[string]config.LLMProvider{
			"LOCAL": {
				Type: "openai-compatible", ID: "LOCAL", Model: "chat-model",
				API: "chat", Enabled: true, APIKey: "test-key",
			},
		},
		LLMModelLists: map[string][]string{"LOCAL": {"chat-model", "opaque-model"}},
		LLMModels: map[string][]config.LLMModel{"LOCAL": {
			{ID: "chat-model", Task: "chat"},
			{ID: "opaque-model", Task: "embedding"},
		}},
	}

	catalog := NewCatalog(app, nil, nil)
	got, err := catalog.ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Models) != 1 || got.Models[0].ID != "chat-model" {
		t.Fatalf("selectable models: %+v", got.Models)
	}
}

func TestEffortModeContext(t *testing.T) {
	t.Parallel()
	ctx := WithEffortMode(context.Background(), "high")
	got, ok := EffortModeFromContext(ctx)
	if !ok || got != "high" {
		t.Fatalf("%q %v", got, ok)
	}
}
