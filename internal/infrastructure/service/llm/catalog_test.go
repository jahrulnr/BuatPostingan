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

func TestEffortModeContext(t *testing.T) {
	t.Parallel()
	ctx := WithEffortMode(context.Background(), "high")
	got, ok := EffortModeFromContext(ctx)
	if !ok || got != "high" {
		t.Fatalf("%q %v", got, ok)
	}
}
