package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"buatpostingan/internal/config"
)

func TestParseSSEResponsesAssemblesTextAndTools(t *testing.T) {
	raw := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_1","object":"response","status":"in_progress","output":[]}}`,
		"",
		"event: response.output_text.delta",
		`data: {"type":"response.output_text.delta","delta":"hello"}`,
		"",
		"event: response.output_text.delta",
		`data: {"type":"response.output_text.delta","delta":" world"}`,
		"",
		"event: response.output_item.done",
		`data: {"type":"response.output_item.done","output_index":0,"item":{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"output_text","text":"hello world"}]}}`,
		"",
		"event: response.output_item.done",
		`data: {"type":"response.output_item.done","output_index":1,"item":{"id":"fc_1","type":"function_call","call_id":"call_1","name":"search_docs","arguments":"{\"query\":\"kafka\"}"}}`,
		"",
		"event: response.reasoning_summary_text.done",
		`data: {"type":"response.reasoning_summary_text.done","text":"thinking…"}`,
		"",
		"event: response.completed",
		`data: {"type":"response.completed","response":{"id":"resp_1","object":"response","status":"completed","error":null}}`,
		"",
	}, "\n")

	payload, err := parseSSEToPayload(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	p := config.LLMProvider{ID: "OPENROUTER", Model: "m", API: "responses"}
	res := parseResponsesPayload(p, payload)
	if res.Text != "hello world" {
		t.Fatalf("text=%q", res.Text)
	}
	if len(res.ToolCalls) != 1 || res.ToolCalls[0].Name != "search_docs" {
		t.Fatalf("toolCalls=%#v", res.ToolCalls)
	}
	if res.ToolCalls[0].Arguments["query"] != "kafka" {
		t.Fatalf("args=%#v", res.ToolCalls[0].Arguments)
	}
	if !strings.Contains(res.Reasoning, "thinking") {
		t.Fatalf("reasoning=%q", res.Reasoning)
	}
}

func TestParseSSEChatCompletionChunks(t *testing.T) {
	raw := strings.Join([]string{
		`: OPENROUTER PROCESSING`,
		"",
		`data: {"object":"chat.completion.chunk","choices":[{"delta":{"role":"assistant","content":"Hi","reasoning":"plan "}}]}`,
		"",
		`data: {"object":"chat.completion.chunk","choices":[{"delta":{"content":"!","reasoning":"done"}}]}`,
		"",
		`data: [DONE]`,
		"",
	}, "\n")
	payload, err := parseSSEToPayload(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	p := config.LLMProvider{ID: "OPENROUTER", Model: "m", API: "chat"}
	res := parseChatCompletionPayload(p, payload)
	if res.Text != "Hi!" {
		t.Fatalf("text=%q", res.Text)
	}
	if res.Reasoning != "plan done" {
		t.Fatalf("reasoning=%q", res.Reasoning)
	}
}

func TestPostJSONParsesResponsesSSE(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accept := r.Header.Get("Accept")
		if !strings.Contains(accept, "text/event-stream") {
			t.Errorf("Accept should allow event-stream, got %q", accept)
		}
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		if body["stream"] != true {
			t.Errorf("stream want true, got %#v", body["stream"])
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, strings.Join([]string{
			"event: response.output_item.done",
			`data: {"type":"response.output_item.done","item":{"type":"message","content":[{"type":"output_text","text":"from sse"}]}}`,
			"",
			"event: response.completed",
			`data: {"type":"response.completed","response":{"status":"completed"}}`,
			"",
		}, "\n"))
	}))
	defer srv.Close()

	c := NewClient(Config{RetryStatuses: []int{429, 500}})
	c.cfg.Providers = map[string]config.LLMProvider{
		"OPENROUTER": {
			ID: "OPENROUTER", BaseURL: srv.URL, APIKey: "test-key", Model: "m", API: "responses", TimeoutSec: 5, MaxOutputTokens: 64,
		},
	}
	c.cfg.ActiveProvider = "OPENROUTER"
	res, err := c.Chat(context.Background(), []map[string]any{{"role": "user", "content": "hi"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "from sse" {
		t.Fatalf("text=%q", res.Text)
	}
}

func TestPostJSONNonStreamJSONFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object": "response",
			"output": []any{
				map[string]any{
					"type": "message",
					"content": []any{
						map[string]any{"type": "output_text", "text": "json path"},
					},
				},
			},
			"usage": map[string]any{"input_tokens": 1, "output_tokens": 2},
		})
	}))
	defer srv.Close()

	c := NewClient(Config{})
	c.cfg.Providers = map[string]config.LLMProvider{
		"OPENROUTER": {
			ID: "OPENROUTER", BaseURL: srv.URL, APIKey: "k", Model: "m", API: "responses", TimeoutSec: 5, MaxOutputTokens: 64,
		},
	}
	c.cfg.ActiveProvider = "OPENROUTER"
	res, err := c.Chat(context.Background(), []map[string]any{{"role": "user", "content": "hi"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "json path" {
		t.Fatalf("text=%q", res.Text)
	}
	if res.Usage.InputTokens != 1 || res.Usage.OutputTokens != 2 {
		t.Fatalf("usage=%#v", res.Usage)
	}
}

func TestParseResponsesNonStreamJSON(t *testing.T) {
	payload := map[string]any{
		"object": "response",
		"output": []any{
			map[string]any{
				"type": "function_call",
				"call_id": "c1",
				"name": "search_docs",
				"arguments": `{"query":"sse"}`,
			},
			map[string]any{
				"type": "message",
				"content": []any{
					map[string]any{"type": "output_text", "text": "ok"},
				},
			},
			map[string]any{
				"type": "reasoning",
				"summary": []any{
					map[string]any{"type": "summary_text", "text": "why"},
				},
			},
		},
	}
	res := parseResponsesPayload(config.LLMProvider{ID: "P", Model: "m", API: "responses"}, payload)
	if res.Text != "ok" {
		t.Fatalf("text=%q", res.Text)
	}
	if len(res.ToolCalls) != 1 || res.ToolCalls[0].Arguments["query"] != "sse" {
		t.Fatalf("tools=%#v", res.ToolCalls)
	}
	if res.Reasoning != "why" {
		t.Fatalf("reasoning=%q", res.Reasoning)
	}
}

func TestPostJSONNonJSONIncludesSnippet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("error: upstream exploded"))
	}))
	defer srv.Close()

	c := NewClient(Config{})
	p := config.LLMProvider{ID: "OPENROUTER", BaseURL: srv.URL, APIKey: "k", TimeoutSec: 5}
	_, err := c.postJSON(context.Background(), p, "chat/completions", map[string]any{"model": "m"})
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "BAD_BODY") || !strings.Contains(msg, "error: upstream exploded") {
		t.Fatalf("want BAD_BODY + body snippet, got %q", msg)
	}
}

func TestChatViaResponsesFallsBackToChatCompletionShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "gen-1",
			"object": "chat.completion",
			"choices": []any{
				map[string]any{
					"message": map[string]any{
						"role":    "assistant",
						"content": "hello from proxy",
					},
				},
			},
			"usage": map[string]any{"prompt_tokens": 1, "completion_tokens": 2},
		})
	}))
	defer srv.Close()

	c := NewClient(Config{})
	c.cfg.Providers = map[string]config.LLMProvider{
		"OPENROUTER": {
			ID: "OPENROUTER", BaseURL: srv.URL, APIKey: "k", Model: "m", API: "responses", TimeoutSec: 5, MaxOutputTokens: 64,
		},
	}
	c.cfg.ActiveProvider = "OPENROUTER"
	res, err := c.Chat(context.Background(), []map[string]any{{"role": "user", "content": "hi"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "hello from proxy" {
		t.Fatalf("text=%q", res.Text)
	}
}

func TestLooksLikeSSE(t *testing.T) {
	if !looksLikeSSE("text/event-stream", []byte("{}")) {
		t.Fatal("ctype should match")
	}
	if !looksLikeSSE("application/json", []byte("event: foo\ndata: {}\n")) {
		t.Fatal("event: prefix should match")
	}
	if looksLikeSSE("application/json", []byte(`{"ok":true}`)) {
		t.Fatal("json should not match")
	}
}

func TestToResponsesRequestIncludesFunctionCallOutput(t *testing.T) {
	p := config.LLMProvider{ID: "OPENROUTER", Model: "m", API: "responses", MaxOutputTokens: 64}
	messages := []map[string]any{
		{"role": "user", "content": "list docs"},
		{
			"role":    "assistant",
			"content": nil,
			"tool_calls": []any{
				map[string]any{
					"id":   "call_1",
					"type": "function",
					"function": map[string]any{
						"name":      "list_dir",
						"arguments": `{"path":""}`,
					},
				},
			},
		},
		{
			"role":         "tool",
			"tool_call_id": "call_1",
			"content":      `{"ok":true,"tool":"list_dir","data":{"listing":"total 0\n. ..","entries":[],"total":0}}`,
		},
	}
	body := toResponsesRequest(p, messages, nil)
	input, _ := body["input"].([]map[string]any)
	if len(input) < 3 {
		t.Fatalf("input=%#v", input)
	}
	var sawCall, sawOut bool
	for _, item := range input {
		switch item["type"] {
		case "function_call":
			sawCall = true
			if item["call_id"] != "call_1" || item["name"] != "list_dir" {
				t.Fatalf("function_call=%#v", item)
			}
		case "function_call_output":
			sawOut = true
			if item["call_id"] != "call_1" {
				t.Fatalf("output call_id=%#v", item["call_id"])
			}
			out, _ := item["output"].(string)
			if !strings.Contains(out, "listing") {
				t.Fatalf("output missing listing: %q", out)
			}
		}
	}
	if !sawCall || !sawOut {
		t.Fatalf("want function_call + function_call_output, input=%#v", input)
	}
}
