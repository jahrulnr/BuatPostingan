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

func TestHeuristicModelSupportsImage(t *testing.T) {
	cases := []struct {
		id   string
		want bool
	}{
		{"xiaomi/mimo-v2.5", true},
		{"openai/gpt-4o", true},
		{"openai/gpt-4.1", true},
		{"anthropic/claude-sonnet-4", true},
		{"google/gemini-2.5-flash", true},
		{"qwen/qwen2.5-vl-72b", true},
		{"deepseek/deepseek-chat", false},
		{"deepseek/deepseek-v3.2", false},
		{"", false},
		{"meta-llama/llama-3.3-70b", false},
	}
	for _, tc := range cases {
		if got := HeuristicModelSupportsImage(tc.id); got != tc.want {
			t.Fatalf("%s: got %v want %v", tc.id, got, tc.want)
		}
	}
}

func TestVisionPolicyModes(t *testing.T) {
	cfg := Config{
		Vision:         VisionOff,
		ActiveProvider: "OPENROUTER",
		Providers: map[string]config.LLMProvider{
			"OPENROUTER": {ID: "OPENROUTER", Model: "xiaomi/mimo-v2.5"},
		},
	}
	p := NewVisionPolicy(cfg)
	if p.AllowPixels(context.Background()) {
		t.Fatal("off must deny")
	}
	cfg.Vision = VisionOn
	p = NewVisionPolicy(cfg)
	if !p.AllowPixels(context.Background()) {
		t.Fatal("on must allow")
	}
}

func TestVisionPolicyAutoFromModelsAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id": "xiaomi/mimo-v2.5",
					"architecture": map[string]any{
						"modality":         "text+image+audio+video->text",
						"input_modalities": []string{"text", "image", "audio", "video"},
					},
				},
				{
					"id": "deepseek/deepseek-chat",
					"architecture": map[string]any{
						"modality":         "text->text",
						"input_modalities": []string{"text"},
					},
				},
			},
		})
	}))
	defer srv.Close()

	base := strings.TrimRight(srv.URL, "/")
	cfg := Config{
		Vision:         VisionAuto,
		ActiveProvider: "OPENROUTER",
		Providers: map[string]config.LLMProvider{
			"OPENROUTER": {
				ID: "OPENROUTER", BaseURL: base, APIKey: "k", Model: "xiaomi/mimo-v2.5",
			},
		},
	}
	p := NewVisionPolicy(cfg)
	p.http = srv.Client()
	if !p.AllowPixels(context.Background()) {
		t.Fatal("mimo should allow via API")
	}
	// cached second call
	if !p.AllowPixels(context.Background()) {
		t.Fatal("cached mimo")
	}

	cfg.Providers["OPENROUTER"] = config.LLMProvider{
		ID: "OPENROUTER", BaseURL: base, APIKey: "k", Model: "deepseek/deepseek-chat",
	}
	p2 := NewVisionPolicy(cfg)
	p2.http = srv.Client()
	if p2.AllowPixels(context.Background()) {
		t.Fatal("deepseek text-only must deny")
	}
}

func TestVisionPolicyAutoHeuristicFallback(t *testing.T) {
	cfg := Config{
		Vision:         VisionAuto,
		ActiveProvider: "OPENROUTER",
		Providers: map[string]config.LLMProvider{
			"OPENROUTER": {ID: "OPENROUTER", Model: "openai/gpt-4o"}, // no BaseURL → heuristic
		},
	}
	p := NewVisionPolicy(cfg)
	if !p.AllowPixels(context.Background()) {
		t.Fatal("gpt-4o heuristic should allow")
	}
	cfg.Providers["OPENROUTER"] = config.LLMProvider{ID: "OPENROUTER", Model: "deepseek/deepseek-r1"}
	p = NewVisionPolicy(cfg)
	if p.AllowPixels(context.Background()) {
		t.Fatal("deepseek heuristic should deny")
	}
}

func TestFromAppIncludesVision(t *testing.T) {
	cfg := FromApp(config.Config{LLMVision: "OFF", LLMStream: true})
	if cfg.Vision != VisionOff {
		t.Fatalf("vision=%q", cfg.Vision)
	}
}
