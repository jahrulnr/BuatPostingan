package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"buatpostingan/internal/config"
)

func TestClaudeMessagesRequestAndResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/messages" {
			t.Errorf("path=%q", r.URL.Path)
		}
		if got := r.Header.Get("x-api-key"); got != "anthropic-key" {
			t.Errorf("x-api-key=%q", got)
		}
		if got := r.Header.Get("anthropic-version"); got == "" {
			t.Error("anthropic-version missing")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["stream"] != false {
			t.Errorf("messages API should use non-stream MVP, got stream=%v", body["stream"])
		}
		if body["system"] != "follow the rules" {
			t.Errorf("system=%#v", body["system"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"type": "message",
			"role": "assistant",
			"content": []any{
				map[string]any{"type": "text", "text": "I will check."},
				map[string]any{
					"type": "tool_use", "id": "toolu_1", "name": "page_list",
					"input": map[string]any{},
				},
			},
			"stop_reason": "tool_use",
			"usage":       map[string]any{"input_tokens": 12, "output_tokens": 7},
		})
	}))
	defer srv.Close()

	client := NewClient(Config{
		ActiveProvider: "CLAUDE",
		Providers: map[string]config.LLMProvider{
			"CLAUDE": {
				ID: "CLAUDE", BaseURL: srv.URL, APIKey: "anthropic-key",
				Model: "claude-sonnet", API: "messages", MaxOutputTokens: 1024,
			},
		},
	})
	got, err := client.Chat(context.Background(), []map[string]any{
		{"role": "system", "content": "follow the rules"},
		{"role": "user", "content": "list pages"},
	}, []map[string]any{{
		"name":       "page_list",
		"parameters": map[string]any{"type": "object", "properties": map[string]any{}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "I will check." || len(got.ToolCalls) != 1 || got.ToolCalls[0].Name != "page_list" {
		t.Fatalf("result=%+v", got)
	}
	if got.Usage.InputTokens != 12 || got.Usage.OutputTokens != 7 {
		t.Fatalf("usage=%+v", got.Usage)
	}
}

func TestKeyOptionalLocalProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("unexpected auth header %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{
				"message": map[string]any{"content": "local ok"},
			}},
		})
	}))
	defer srv.Close()
	stream := false
	client := NewClient(Config{
		ActiveProvider: "OMNIROUTE",
		Stream:         &stream,
		Providers: map[string]config.LLMProvider{
			"OMNIROUTE": {
				ID: "OMNIROUTE", BaseURL: srv.URL, Model: "auto",
				API: "chat", APIKeyOptional: true,
			},
		},
	})
	got, err := client.Chat(context.Background(), []map[string]any{{"role": "user", "content": "hi"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "local ok" {
		t.Fatalf("text=%q", got.Text)
	}
}

func TestMessagesUserContentConvertsDataURLImage(t *testing.T) {
	got, ok := toMessagesUserContent([]any{
		map[string]any{"type": "text", "text": "describe"},
		map[string]any{
			"type":      "image_url",
			"image_url": map[string]any{"url": "data:image/png;base64,aGVsbG8="},
		},
	}).([]any)
	if !ok || len(got) != 2 {
		t.Fatalf("content=%#v", got)
	}
	image, _ := got[1].(map[string]any)
	source, _ := image["source"].(map[string]any)
	if source["media_type"] != "image/png" || source["data"] != "aGVsbG8=" {
		t.Fatalf("source=%#v", source)
	}
}
