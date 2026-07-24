package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"buatpostingan/internal/config"
)

func TestHeuristicModelSupportsEffort(t *testing.T) {
	cases := []struct {
		id   string
		want bool
	}{
		{"openai/o3-mini", true},
		{"openai/gpt-5", true},
		{"openai/gpt-5.2", true},
		{"deepseek/deepseek-r1", true},
		{"anthropic/claude-sonnet-4", true},
		{"google/gemini-2.5-flash", true},
		{"qwen/qwq-32b", true},
		{"openai/gpt-4o", false},
		{"openai/gpt-4o-mini", false},
		{"deepseek/deepseek-chat", false},
		{"xiaomi/mimo-v2.5", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := HeuristicModelSupportsEffort(tc.id); got != tc.want {
			t.Fatalf("%s: got %v want %v", tc.id, got, tc.want)
		}
	}
}

func TestEffortPolicyExplicitOmitsUnsupported(t *testing.T) {
	cfg := Config{
		Effort:         EffortHigh,
		ActiveProvider: "OPENROUTER",
		Providers: map[string]config.LLMProvider{
			"OPENROUTER": {ID: "OPENROUTER", Model: "deepseek/deepseek-chat"}, // no BaseURL → heuristic deny
		},
	}
	p := NewEffortPolicy(cfg)
	got := p.ResolveFor(context.Background(), cfg.Providers["OPENROUTER"])
	if got != "" {
		t.Fatalf("unsupported must omit, got %q", got)
	}
}

func TestEffortPolicyExplicitHeuristic(t *testing.T) {
	cfg := Config{
		Effort:         EffortHigh,
		ActiveProvider: "OPENROUTER",
		Providers: map[string]config.LLMProvider{
			"OPENROUTER": {ID: "OPENROUTER", Model: "openai/o3-mini"},
		},
	}
	p := NewEffortPolicy(cfg)
	got := p.ResolveFor(context.Background(), cfg.Providers["OPENROUTER"])
	if got != EffortHigh {
		t.Fatalf("got %q", got)
	}
}

func TestEffortPolicyAutoFromModelsAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id": "openai/o3-mini",
					"supported_parameters": []string{"reasoning", "tools"},
					"reasoning": map[string]any{
						"supported_efforts": []string{"high", "medium", "low"},
						"default_effort":    "medium",
						"mandatory":         false,
					},
				},
				{
					"id": "openai/gpt-4o-mini",
					"supported_parameters": []string{"tools", "temperature"},
				},
				{
					"id": "anthropic/claude-sonnet-4",
					"supported_parameters": []string{"reasoning", "include_reasoning"},
					"reasoning": map[string]any{
						"supported_efforts":   []string{"high", "medium", "low"},
						"default_effort":      "low",
						"supports_max_tokens": true,
					},
				},
				{
					"id": "openai/o1-pro",
					"reasoning": map[string]any{
						"supported_efforts": []string{"high", "medium"},
						"default_effort":    "medium",
						"mandatory":         true,
					},
				},
			},
		})
	}))
	defer srv.Close()

	base := strings.TrimRight(srv.URL, "/")
	prov := func(model string) config.LLMProvider {
		return config.LLMProvider{ID: "OPENROUTER", BaseURL: base, APIKey: "k", Model: model, API: "chat"}
	}

	cfg := Config{
		Effort:         EffortAuto,
		ActiveProvider: "OPENROUTER",
		Providers:      map[string]config.LLMProvider{"OPENROUTER": prov("openai/o3-mini")},
	}
	p := NewEffortPolicy(cfg)
	p.http = srv.Client()
	if got := p.ResolveFor(context.Background(), cfg.Providers["OPENROUTER"]); got != EffortMedium {
		t.Fatalf("auto default_effort: got %q", got)
	}
	// cached
	if got := p.ResolveFor(context.Background(), cfg.Providers["OPENROUTER"]); got != EffortMedium {
		t.Fatalf("cached: got %q", got)
	}

	cfg.Providers["OPENROUTER"] = prov("openai/gpt-4o-mini")
	p2 := NewEffortPolicy(cfg)
	p2.http = srv.Client()
	if got := p2.ResolveFor(context.Background(), cfg.Providers["OPENROUTER"]); got != "" {
		t.Fatalf("text model must omit under auto, got %q", got)
	}

	cfg.Effort = EffortXHigh
	cfg.Providers["OPENROUTER"] = prov("openai/o3-mini")
	p3 := NewEffortPolicy(cfg)
	p3.http = srv.Client()
	if got := p3.ResolveFor(context.Background(), cfg.Providers["OPENROUTER"]); got != EffortHigh {
		t.Fatalf("xhigh should clamp to high, got %q", got)
	}

	cfg.Effort = EffortNone
	cfg.Providers["OPENROUTER"] = prov("openai/o1-pro")
	p4 := NewEffortPolicy(cfg)
	p4.http = srv.Client()
	if got := p4.ResolveFor(context.Background(), cfg.Providers["OPENROUTER"]); got != "" {
		t.Fatalf("mandatory must not get none, got %q", got)
	}
}

func TestEffortPolicyAutoHeuristicFallback(t *testing.T) {
	cfg := Config{
		Effort:         EffortAuto,
		ActiveProvider: "OPENROUTER",
		Providers: map[string]config.LLMProvider{
			"OPENROUTER": {ID: "OPENROUTER", Model: "openai/o3-mini"},
		},
	}
	p := NewEffortPolicy(cfg)
	if got := p.ResolveFor(context.Background(), cfg.Providers["OPENROUTER"]); got != EffortMedium {
		t.Fatalf("heuristic auto → medium, got %q", got)
	}
	cfg.Providers["OPENROUTER"] = config.LLMProvider{ID: "OPENROUTER", Model: "deepseek/deepseek-chat"}
	p = NewEffortPolicy(cfg)
	if got := p.ResolveFor(context.Background(), cfg.Providers["OPENROUTER"]); got != "" {
		t.Fatalf("heuristic deny must omit, got %q", got)
	}
}

func TestApplyEffortShapes(t *testing.T) {
	chat := map[string]any{}
	ApplyEffort(chat, "chat", EffortHigh)
	if chat["reasoning_effort"] != EffortHigh {
		t.Fatalf("chat reasoning_effort=%v", chat["reasoning_effort"])
	}
	r, _ := chat["reasoning"].(map[string]any)
	if r["effort"] != EffortHigh {
		t.Fatalf("chat reasoning.effort=%v", r["effort"])
	}

	resp := map[string]any{}
	ApplyEffort(resp, "responses", EffortLow)
	if _, ok := resp["reasoning_effort"]; ok {
		t.Fatal("responses must not set top-level reasoning_effort")
	}
	rr, _ := resp["reasoning"].(map[string]any)
	if rr["effort"] != EffortLow {
		t.Fatalf("responses reasoning.effort=%v", rr["effort"])
	}

	omit := map[string]any{}
	ApplyEffort(omit, "chat", "")
	if len(omit) != 0 {
		t.Fatalf("empty effort must omit: %#v", omit)
	}
}

func TestFromAppIncludesEffort(t *testing.T) {
	cfg := FromApp(config.Config{LLMEffort: "HIGH", LLMStream: true})
	if cfg.Effort != EffortHigh {
		t.Fatalf("effort=%q", cfg.Effort)
	}
}

func TestClientAppliesEffortOnChatAndResponses(t *testing.T) {
	var gotBodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// EffortPolicy may probe /models; ignore for body assertions.
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotBodies = append(gotBodies, body)
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "responses") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"object": "response",
				"output": []any{
					map[string]any{
						"type": "message",
						"content": []any{
							map[string]any{"type": "output_text", "text": "ok"},
						},
					},
				},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object": "chat.completion",
			"choices": []any{
				map[string]any{"message": map[string]any{"role": "assistant", "content": "ok"}},
			},
		})
	}))
	defer srv.Close()

	streamOff := false
	c := NewClient(Config{
		Effort:         EffortMedium,
		Stream:         &streamOff,
		ActiveProvider: "OPENROUTER",
		Providers: map[string]config.LLMProvider{
			"OPENROUTER": {
				ID: "OPENROUTER", BaseURL: srv.URL, APIKey: "k",
				Model: "openai/o3-mini", API: "chat", TimeoutSec: 5, MaxOutputTokens: 64,
			},
		},
	})
	if _, err := c.Chat(context.Background(), []map[string]any{{"role": "user", "content": "hi"}}, nil); err != nil {
		t.Fatal(err)
	}
	if len(gotBodies) != 1 {
		t.Fatalf("bodies=%d", len(gotBodies))
	}
	if gotBodies[0]["reasoning_effort"] != EffortMedium {
		t.Fatalf("chat body=%#v", gotBodies[0])
	}

	gotBodies = nil
	c.cfg.Providers["OPENROUTER"] = config.LLMProvider{
		ID: "OPENROUTER", BaseURL: srv.URL, APIKey: "k",
		Model: "openai/o3-mini", API: "responses", TimeoutSec: 5, MaxOutputTokens: 64,
	}
	c.effort = NewEffortPolicy(c.cfg)
	if _, err := c.Chat(context.Background(), []map[string]any{{"role": "user", "content": "hi"}}, nil); err != nil {
		t.Fatal(err)
	}
	if len(gotBodies) != 1 {
		t.Fatalf("responses bodies=%d", len(gotBodies))
	}
	rr, _ := gotBodies[0]["reasoning"].(map[string]any)
	if rr["effort"] != EffortMedium {
		t.Fatalf("responses body=%#v", gotBodies[0])
	}
	if _, ok := gotBodies[0]["reasoning_effort"]; ok {
		t.Fatal("responses should not set reasoning_effort")
	}
}
