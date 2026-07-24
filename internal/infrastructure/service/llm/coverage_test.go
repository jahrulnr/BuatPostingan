package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"buatpostingan/internal/config"
)

func TestFromAppAndErrorUnwrap(t *testing.T) {
	cfg := FromApp(config.Config{
		StorageRoot: "/tmp/x", LLMStrategy: "failover", LLMActiveProvider: "A",
		LLMTotalAttemptBudget: 3, LLMCircuitFailureThreshold: 2, LLMCircuitCooldownSec: 10,
		LLMRetryStatuses: []int{429}, LLMProviders: map[string]config.LLMProvider{"A": {ID: "A"}},
		LLMStream: true,
	})
	if cfg.StorageRoot != "/tmp/x" || cfg.Strategy != "failover" || cfg.ActiveProvider != "A" {
		t.Fatalf("%#v", cfg)
	}
	if cfg.Stream == nil || !*cfg.Stream {
		t.Fatal("Stream should be true from FromApp")
	}
	cfgOff := FromApp(config.Config{LLMStream: false})
	if cfgOff.Stream == nil || *cfgOff.Stream {
		t.Fatal("Stream should be false from FromApp")
	}
	cause := errors.New("root")
	err := &Error{Provider: "P", Msg: "fail", Cause: cause}
	if !strings.Contains(err.Error(), "root") {
		t.Fatalf("%q", err.Error())
	}
	if !errors.Is(err, cause) {
		t.Fatal("Unwrap")
	}
	plain := &Error{Provider: "P", Msg: "only"}
	if plain.Error() != "llm[P]: only" {
		t.Fatalf("%q", plain.Error())
	}
}

func TestCircuitStoreOpenAndCooldown(t *testing.T) {
	root := t.TempDir()
	c := newCircuitStore(root, 2, 1)
	if c.failureThreshold != 2 || c.cooldownSec != 1 {
		t.Fatalf("%#v", c)
	}
	// defaults
	c2 := newCircuitStore(root, 0, 0)
	if c2.failureThreshold != 3 || c2.cooldownSec != 60 {
		t.Fatalf("defaults %#v", c2)
	}

	c.record("A", false)
	st := c.read()
	if st["A"].Failures != 1 || st["A"].OpenedAt != nil {
		t.Fatalf("%#v", st["A"])
	}
	c.record("A", false)
	st = c.read()
	if st["A"].Failures != 2 || st["A"].OpenedAt == nil {
		t.Fatalf("should open %#v", st["A"])
	}
	now := time.Now()
	if c.isAvailable("A", st, now) {
		t.Fatal("circuit should be open")
	}
	if !c.isAvailable("A", st, now.Add(2*time.Second)) {
		t.Fatal("cooldown elapsed should allow")
	}
	if !c.isAvailable("missing", st, now) {
		t.Fatal("missing provider available")
	}
	c.record("A", true)
	st = c.read()
	if st["A"].Failures != 0 || st["A"].OpenedAt != nil {
		t.Fatalf("reset %#v", st["A"])
	}

	// corrupt file → empty read
	_ = os.WriteFile(c.path(), []byte("not-json"), 0o644)
	if len(c.read()) != 0 {
		t.Fatal("corrupt should yield empty")
	}
}

func TestRouterCandidatesStrategies(t *testing.T) {
	root := t.TempDir()
	providers := map[string]config.LLMProvider{
		"A": {ID: "A", Enabled: true, APIKey: "k", BaseURL: "http://x", Model: "m", API: "chat"},
		"B": {ID: "B", Enabled: true, APIKey: "k", BaseURL: "http://x", Model: "m", API: "chat"},
		"C": {ID: "C", Enabled: false, APIKey: "k"},
	}
	cfg := Config{
		StorageRoot: root, Strategy: "failover", ActiveProvider: "A",
		Providers: providers, CircuitFailureThreshold: 1, CircuitCooldownSec: 60,
	}
	r := NewRouter(cfg, NewClient(cfg))
	got := r.candidates("")
	if len(got) < 1 || got[0] != "A" {
		t.Fatalf("failover order %#v", got)
	}

	cfg.Strategy = "switch"
	r = NewRouter(cfg, nil)
	got = r.candidates("")
	if len(got) != 1 || got[0] != "A" {
		t.Fatalf("switch %#v", got)
	}

	cfg.Strategy = "round_robin"
	cfg.ActiveProvider = ""
	r = NewRouter(cfg, NewClient(cfg))
	first := r.candidates("")
	second := r.candidates("")
	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("rr %#v %#v", first, second)
	}
	// cursor advances
	raw, _ := os.ReadFile(filepath.Join(root, "llm", "round_robin.cursor"))
	if strings.TrimSpace(string(raw)) == "" {
		t.Fatal("cursor file missing")
	}

	cfg.Strategy = "failover"
	r = NewRouter(cfg, NewClient(cfg))
	pinned := r.candidates("B")
	if pinned[0] != "B" {
		t.Fatalf("pinned %#v", pinned)
	}

	// open circuit for A → still returns enabled when all open (fallback)
	r.circuit.record("A", false)
	r.circuit.record("B", false)
	all := r.candidates("")
	if len(all) != 2 {
		t.Fatalf("all open fallback %#v", all)
	}
}

func TestRouterChatSuccessFailoverAndExhaust(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"slow"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object": "chat.completion",
			"choices": []any{map[string]any{
				"message": map[string]any{"role": "assistant", "content": "ok"},
			}},
		})
	}))
	defer srv.Close()

	root := t.TempDir()
	cfg := Config{
		StorageRoot: root, Strategy: "failover", ActiveProvider: "A",
		TotalAttemptBudget: 4, RetryStatuses: []int{429, 500},
		Providers: map[string]config.LLMProvider{
			"A": {ID: "A", Enabled: true, APIKey: "k", BaseURL: srv.URL, Model: "m", API: "chat", TimeoutSec: 5, MaxAttempts: 2, MaxOutputTokens: 32},
		},
	}
	r := NewRouter(cfg, NewClient(cfg))
	res, err := r.Chat(context.Background(), []map[string]any{{"role": "user", "content": "hi"}}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "ok" {
		t.Fatalf("%#v", res)
	}

	// non-transient stops immediately
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`nope`))
	}))
	defer srv2.Close()
	cfg2 := cfg
	cfg2.Providers = map[string]config.LLMProvider{
		"A": {ID: "A", Enabled: true, APIKey: "k", BaseURL: srv2.URL, Model: "m", API: "chat", TimeoutSec: 5, MaxAttempts: 3},
	}
	r2 := NewRouter(cfg2, NewClient(cfg2))
	_, err = r2.Chat(context.Background(), []map[string]any{{"role": "user", "content": "hi"}}, nil, "")
	if err == nil {
		t.Fatal("want auth error")
	}
	le, ok := err.(*Error)
	if !ok || le.Transient {
		t.Fatalf("want non-transient %#v", err)
	}

	// exhausted — no enabled providers
	cfg3 := Config{StorageRoot: root, Providers: map[string]config.LLMProvider{
		"A": {ID: "A", Enabled: false, APIKey: "k"},
	}}
	r3 := NewRouter(cfg3, NewClient(cfg3))
	_, err = r3.Chat(context.Background(), nil, nil, "")
	if err == nil || !strings.Contains(err.Error(), "exhausted") {
		t.Fatalf("err=%v", err)
	}
}

func TestChatViaCompletionsWithTools(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		tools, _ := body["tools"].([]any)
		if len(tools) != 1 {
			t.Errorf("tools=%#v", tools)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object": "chat.completion",
			"choices": []any{map[string]any{
				"message": map[string]any{
					"role":    "assistant",
					"content": nil,
					"tool_calls": []any{
						map[string]any{
							"id": "call_x", "type": "function",
							"function": map[string]any{"name": "search_docs", "arguments": `{"query":"q"}`},
						},
						map[string]any{"id": "", "function": map[string]any{"name": "bad", "arguments": "{"}},
					},
				},
			}},
			"usage": map[string]any{
				"prompt_tokens": 1, "completion_tokens": 2,
				"prompt_tokens_details": map[string]any{"cached_tokens": 3, "cache_write_tokens": 4},
				"output_tokens_details": map[string]any{"reasoning_tokens": 5},
			},
		})
	}))
	defer srv.Close()

	c := NewClient(Config{RetryStatuses: []int{429}})
	c.cfg.Providers = map[string]config.LLMProvider{
		"CHAT": {ID: "CHAT", BaseURL: srv.URL, APIKey: "k", Model: "m", API: "chat", TimeoutSec: 5, MaxOutputTokens: 64},
	}
	c.cfg.ActiveProvider = "CHAT"
	tools := []map[string]any{
		{"type": "function", "function": map[string]any{
			"name": "search_docs", "description": "d",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string"},
					"top_k": map[string]any{"type": "integer"},
				},
				"required": []any{"query"},
			},
		}},
	}
	res, err := c.Chat(context.Background(), []map[string]any{{"role": "user", "content": "hi"}}, tools)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.ToolCalls) < 1 || res.ToolCalls[0].Name != "search_docs" {
		t.Fatalf("%#v", res.ToolCalls)
	}
	if res.Usage.CachedInputTokens != 3 || res.Usage.ReasoningOutputTokens != 5 {
		t.Fatalf("usage %#v", res.Usage)
	}
}

func TestChatWithProviderErrorsAndHTTPStatuses(t *testing.T) {
	c := NewClient(Config{})
	_, err := c.ChatWithProvider(context.Background(), "missing", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "provider missing") {
		t.Fatalf("%v", err)
	}
	c.cfg.Providers = map[string]config.LLMProvider{"P": {ID: "P", APIKey: ""}}
	_, err = c.ChatWithProvider(context.Background(), "P", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "API key missing") {
		t.Fatalf("%v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("fail"))
	}))
	defer srv.Close()
	c = NewClient(Config{RetryStatuses: []int{500}})
	p := config.LLMProvider{ID: "P", BaseURL: srv.URL, APIKey: "k", TimeoutSec: 5}
	_, err = c.postJSON(context.Background(), p, "chat/completions", map[string]any{"model": "m"})
	le, ok := err.(*Error)
	if !ok || !le.Transient || le.Status != 500 {
		t.Fatalf("%#v", err)
	}
	if !c.isTransient(500) || c.isTransient(400) {
		t.Fatal("isTransient")
	}

	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv2.Close()
	p.BaseURL = srv2.URL
	_, err = c.postJSON(context.Background(), p, "chat/completions", map[string]any{"model": "m"})
	if err == nil || !strings.Contains(err.Error(), "empty body") {
		t.Fatalf("%v", err)
	}
}

func TestNormalizeToolsAndParameters(t *testing.T) {
	tools := []map[string]any{
		{"name": "bare", "description": "d", "parameters": map[string]any{
			"properties": map[string]any{
				"n": map[string]any{"type": "number"},
				"b": map[string]any{"type": "boolean"},
				"s": "not-a-map",
			},
			"required": []any{"n"},
		}},
		{"function": map[string]any{"name": "fn"}},
	}
	chat := normalizeChatTools(tools)
	if len(chat) != 2 {
		t.Fatalf("%#v", chat)
	}
	fn := chat[0]["function"].(map[string]any)
	params := fn["parameters"].(map[string]any)
	props := params["properties"].(map[string]any)
	// optional boolean → tolerant types including null/string
	bType := props["b"].(map[string]any)["type"]
	if _, ok := bType.([]any); !ok {
		t.Fatalf("optional type %#v", bType)
	}
	resp := toResponsesTools(tools)
	if len(resp) != 2 || resp[0]["name"] != "bare" {
		t.Fatalf("%#v", resp)
	}
}

func TestToResponsesRequestSystemAndAssistant(t *testing.T) {
	p := config.LLMProvider{ID: "P", Model: "m", MaxOutputTokens: 10}
	body := toResponsesRequest(p, []map[string]any{
		{"role": "system", "content": "sys"},
		{"role": "developer", "content": "dev"},
		{"role": "assistant", "content": "hi"},
		{"role": "user", "content": "u"},
	}, []map[string]any{{"function": map[string]any{"name": "t"}}}, true)
	if !strings.Contains(body["instructions"].(string), "sys") {
		t.Fatalf("%#v", body["instructions"])
	}
	input := body["input"].([]map[string]any)
	if len(input) != 2 {
		t.Fatalf("%#v", input)
	}
	if body["tools"] == nil {
		t.Fatal("tools missing")
	}
}

func TestAsIntAndMapUsageVariants(t *testing.T) {
	if asInt(nil, "x", int64(7), 3.0) != 7 {
		t.Fatal("asInt int64")
	}
	if asInt(json.Number("9")) != 9 {
		t.Fatal("asInt Number")
	}
	if asInt(3) != 3 {
		t.Fatal("asInt int")
	}
	u := mapUsage(map[string]any{
		"input_tokens": 1.0, "output_tokens": 2.0,
		"input_tokens_details":  map[string]any{"cached_tokens": 8.0, "cache_write_tokens": 9.0},
		"output_tokens_details": map[string]any{"reasoning_tokens": 1.0},
	})
	if u.InputTokens != 1 || u.CachedInputTokens != 8 || u.CacheWriteTokens != 9 {
		t.Fatalf("%#v", u)
	}
	if mapUsage(nil).InputTokens != 0 {
		t.Fatal("nil usage")
	}
}

func TestLooksLikeChatCompletion(t *testing.T) {
	if looksLikeChatCompletion(nil) {
		t.Fatal("nil")
	}
	if !looksLikeChatCompletion(map[string]any{"object": "chat.completion"}) {
		t.Fatal("object")
	}
	if !looksLikeChatCompletion(map[string]any{"choices": []any{}}) {
		t.Fatal("choices")
	}
	if looksLikeChatCompletion(map[string]any{"choices": []any{}, "output": []any{}}) {
		t.Fatal("has output")
	}
}

func TestParseResponsesOutputTextAndArgsMap(t *testing.T) {
	res := parseResponsesPayload(config.LLMProvider{ID: "P", Model: "m", API: "responses"}, map[string]any{
		"output_text": "fallback",
		"output": []any{
			"skip",
			map[string]any{"type": "function_call", "id": "i1", "name": "t", "arguments": map[string]any{"a": 1}},
			map[string]any{"type": "message", "content": []any{
				map[string]any{"type": "text", "text": ""},
				nil,
			}},
			map[string]any{"type": "reasoning", "text": "direct", "content": []any{
				map[string]any{"text": "block"},
			}},
		},
	})
	if res.Text != "fallback" {
		t.Fatalf("text=%q", res.Text)
	}
	if len(res.ToolCalls) != 1 || res.ToolCalls[0].Arguments["a"] != 1 {
		t.Fatalf("%#v", res.ToolCalls)
	}
	if !strings.Contains(res.Reasoning, "direct") || !strings.Contains(res.Reasoning, "block") {
		t.Fatalf("reasoning=%q", res.Reasoning)
	}
}

func TestParseSSEResponsesFailedAndChatTools(t *testing.T) {
	rawFail := strings.Join([]string{
		"event: response.failed",
		`data: {"type":"response.failed","response":{"error":{"message":"nope"}}}`,
		"",
	}, "\n")
	_, err := parseSSEToPayload(strings.NewReader(rawFail))
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("%v", err)
	}

	rawChatTools := strings.Join([]string{
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"c1","function":{"name":"search_docs","arguments":"{\"q\":"}}]}}]}`,
		"",
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"x\"}"}}]}}]}`,
		"",
		`data: {"choices":[{"message":{"content":"ignored-when-delta"}}]}`,
		"",
		`data: {"usage":{"prompt_tokens":1},"choices":[{"delta":{}}]}`,
		"",
		`data: [DONE]`,
		"",
	}, "\n")
	payload, err := parseSSEToPayload(strings.NewReader(rawChatTools))
	if err != nil {
		t.Fatal(err)
	}
	res := parseChatCompletionPayload(config.LLMProvider{ID: "P", Model: "m", API: "chat"}, payload)
	if len(res.ToolCalls) != 1 || res.ToolCalls[0].Name != "search_docs" {
		t.Fatalf("%#v", res)
	}

	// unrecognized stream
	_, err = parseSSEToPayload(strings.NewReader("data: {\"foo\":1}\n\n"))
	if err == nil || !strings.Contains(err.Error(), "no recognizable") {
		t.Fatalf("%v", err)
	}
}

func TestParseSSEResponsesDeltasSynthesize(t *testing.T) {
	raw := strings.Join([]string{
		"event: response.output_text.delta",
		`data: {"type":"response.output_text.delta","delta":"hi"}`,
		"",
		"event: response.reasoning_summary_text.delta",
		`data: {"type":"response.reasoning_summary_text.delta","delta":"r"}`,
		"",
		"event: response.output_text.done",
		`data: {"type":"response.output_text.done","text":"hi done"}`,
		"",
		"event: response.completed",
		`data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":1},"error":null}}`,
		"",
	}, "\n")
	payload, err := parseSSEToPayload(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if payload["output_text"] != "hi done" {
		t.Fatalf("%#v", payload)
	}
	res := parseResponsesPayload(config.LLMProvider{ID: "P", Model: "m", API: "responses"}, payload)
	if res.Text != "hi done" {
		t.Fatalf("%q", res.Text)
	}
	if !strings.Contains(res.Reasoning, "r") {
		t.Fatalf("reasoning=%q", res.Reasoning)
	}
}

func TestTruncateBody(t *testing.T) {
	if truncateBody([]byte("  a\nb  "), 10) != "a b" {
		t.Fatal("normalize")
	}
	long := strings.Repeat("x", 20)
	if len(truncateBody([]byte(long), 5)) != 5 {
		t.Fatal("truncate")
	}
}

func TestParseSSEResponsesIncompleteErrorAndBareFailed(t *testing.T) {
	raw := strings.Join([]string{
		"event: response.output_item.done",
		`data: {"type":"response.output_item.done","item":{"type":"reasoning","summary":[{"type":"summary_text","text":"keep"}]}}`,
		"",
		"event: response.incomplete",
		`data: {"type":"response.incomplete","response":{"status":"incomplete","output":[{"type":"message","content":[{"type":"output_text","text":"partial"}]}],"usage":{"input_tokens":2},"error":{"message":""}}}`,
		"",
	}, "\n")
	payload, err := parseSSEToPayload(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	res := parseResponsesPayload(config.LLMProvider{ID: "P", Model: "m", API: "responses"}, payload)
	if res.Text != "partial" {
		t.Fatalf("text=%q", res.Text)
	}

	_, err = parseSSEToPayload(strings.NewReader(strings.Join([]string{
		"event: response.failed",
		`data: {"type":"response.failed","response":{}}`,
		"",
	}, "\n")))
	if err == nil || !strings.Contains(err.Error(), "response.failed") {
		t.Fatalf("%v", err)
	}

	// chat chunk without object but with choices+delta
	payload, err = parseSSEToPayload(strings.NewReader(`data: {"choices":[{"delta":{"content":"z"}}]}`+"\n\n"))
	if err != nil {
		t.Fatal(err)
	}
	if parseChatCompletionPayload(config.LLMProvider{ID: "P", Model: "m"}, payload).Text != "z" {
		t.Fatalf("%#v", payload)
	}
}

func TestRouterBudgetExhaustReturnsLast(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("slow"))
	}))
	defer srv.Close()
	root := t.TempDir()
	cfg := Config{
		StorageRoot: root, Strategy: "failover", ActiveProvider: "A",
		TotalAttemptBudget: 2, RetryStatuses: []int{429},
		Providers: map[string]config.LLMProvider{
			"A": {ID: "A", Enabled: true, APIKey: "k", BaseURL: srv.URL, Model: "m", API: "chat", TimeoutSec: 5, MaxAttempts: 5},
			"B": {ID: "B", Enabled: true, APIKey: "k", BaseURL: srv.URL, Model: "m", API: "chat", TimeoutSec: 5, MaxAttempts: 5},
		},
	}
	r := NewRouter(cfg, NewClient(cfg))
	_, err := r.Chat(context.Background(), []map[string]any{{"role": "user", "content": "hi"}}, nil, "")
	if err == nil {
		t.Fatal("want error")
	}
}

func TestLooksLikeSSEDataPrefix(t *testing.T) {
	if !looksLikeSSE("application/json", []byte("data: {}\n")) {
		t.Fatal("data: prefix")
	}
}

func TestChatViaResponsesWithToolsBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		if body["tools"] == nil {
			t.Errorf("tools missing in %#v", body)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: response.output_item.done\ndata: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"message\",\"content\":[{\"type\":\"output_text\",\"text\":\"t\"}]}}\n\n")
		_, _ = io.WriteString(w, "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n")
	}))
	defer srv.Close()
	c := NewClient(Config{})
	c.cfg.Providers = map[string]config.LLMProvider{
		"R": {ID: "R", BaseURL: srv.URL, APIKey: "k", Model: "m", API: "responses", TimeoutSec: 5, MaxOutputTokens: 16},
	}
	c.cfg.ActiveProvider = "R"
	res, err := c.Chat(context.Background(), []map[string]any{
		{"role": "system", "content": "s"},
		{"role": "user", "content": "u"},
	}, []map[string]any{{"function": map[string]any{"name": "search_docs", "parameters": map[string]any{}}}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "t" {
		t.Fatalf("%q", res.Text)
	}
}
