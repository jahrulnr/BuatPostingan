package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"buatpostingan/internal/config"
	"buatpostingan/internal/domain/service"
	"buatpostingan/internal/pkg/idgen"
	"buatpostingan/internal/pkg/logging"
)

// Client talks to OpenAI-compatible chat/completions (and responses).
type Client struct {
	mu     sync.RWMutex
	cfg    Config
	http   *http.Client
	retry  map[int]struct{}
	effort *EffortPolicy
}

func NewClient(cfg Config) *Client {
	retry := make(map[int]struct{}, len(cfg.RetryStatuses)+1)
	for _, s := range cfg.RetryStatuses {
		retry[s] = struct{}{}
	}
	retry[413] = struct{}{}
	return &Client{
		cfg:    cfg,
		http:   &http.Client{},
		retry:  retry,
		effort: NewEffortPolicy(cfg),
	}
}

// Reload swaps provider config after settings save (hot path).
func (c *Client) Reload(cfg Config) {
	c.mu.Lock()
	defer c.mu.Unlock()
	retry := make(map[int]struct{}, len(cfg.RetryStatuses)+1)
	for _, s := range cfg.RetryStatuses {
		retry[s] = struct{}{}
	}
	retry[413] = struct{}{}
	c.cfg = cfg
	c.retry = retry
	c.effort = NewEffortPolicy(cfg)
}

func (c *Client) Chat(ctx context.Context, messages []map[string]any, tools []map[string]any) (service.LLMResult, error) {
	c.mu.RLock()
	active := c.cfg.ActiveProvider
	c.mu.RUnlock()
	return c.ChatWithProvider(ctx, active, messages, tools)
}

func (c *Client) ChatWithProvider(ctx context.Context, providerID string, messages []map[string]any, tools []map[string]any) (service.LLMResult, error) {
	c.mu.RLock()
	p, ok := c.cfg.Providers[providerID]
	c.mu.RUnlock()
	if !ok {
		return service.LLMResult{}, &Error{Provider: providerID, Msg: "provider missing", Transient: false}
	}
	if strings.TrimSpace(p.APIKey) == "" && !p.APIKeyOptional {
		return service.LLMResult{}, &Error{Provider: providerID, Msg: "API key missing", Transient: false}
	}
	if override, ok := ModelOverrideFromContext(ctx); ok {
		p.Model = override
	}
	if p.API == "responses" {
		return c.chatViaResponses(ctx, p, messages, tools)
	}
	if p.API == "messages" {
		return c.chatViaMessages(ctx, p, messages, tools)
	}
	return c.chatViaCompletions(ctx, p, messages, tools)
}

func (c *Client) wantStream() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.cfg.Stream == nil {
		return true
	}
	return *c.cfg.Stream
}

func (c *Client) chatViaCompletions(ctx context.Context, p config.LLMProvider, messages []map[string]any, tools []map[string]any) (service.LLMResult, error) {
	body := map[string]any{
		"model":       p.Model,
		"messages":    messages,
		"tool_choice": "auto",
		"max_tokens":  p.MaxOutputTokens,
		"stream":      c.wantStream(),
	}
	if len(tools) > 0 {
		body["tools"] = normalizeChatTools(tools)
	}
	c.applyEffort(ctx, p, body)
	payload, err := c.postJSON(ctx, p, "chat/completions", body)
	if err != nil {
		return service.LLMResult{}, err
	}
	logging.Warn(ctx, "webchat.llm.response",
		"provider", p.ID,
		"api", "chat",
		"path", "chat/completions",
		"stream", c.wantStream(),
		"hasChoices", payload != nil && payload["choices"] != nil,
	)
	return parseChatCompletionPayload(p, payload), nil
}

func (c *Client) chatViaResponses(ctx context.Context, p config.LLMProvider, messages []map[string]any, tools []map[string]any) (service.LLMResult, error) {
	body := toResponsesRequest(p, messages, tools, c.wantStream())
	c.applyEffort(ctx, p, body)
	payload, err := c.postJSON(ctx, p, "responses", body)
	if err != nil {
		return service.LLMResult{}, err
	}
	logging.Warn(ctx, "webchat.llm.response",
		"provider", p.ID,
		"api", "responses",
		"path", "responses",
		"stream", c.wantStream(),
		"hasChoices", payload != nil && payload["choices"] != nil,
		"hasOutput", payload != nil && payload["output"] != nil,
	)
	// Some proxies may still return chat.completion JSON (e.g. stream=false path).
	if looksLikeChatCompletion(payload) {
		logging.Warn(ctx, "webchat.llm.responses_shape_is_chat", "provider", p.ID)
		return parseChatCompletionPayload(p, payload), nil
	}
	return parseResponsesPayload(p, payload), nil
}

func (c *Client) chatViaMessages(ctx context.Context, p config.LLMProvider, messages []map[string]any, tools []map[string]any) (service.LLMResult, error) {
	body := toMessagesRequest(p, messages, tools)
	// Anthropic streaming uses a distinct event protocol. Keep this rail
	// non-streaming until that protocol has its own parser.
	payload, err := c.postJSONStream(ctx, p, "messages", body, false, false)
	if err != nil {
		return service.LLMResult{}, err
	}
	return parseMessagesPayload(p, payload), nil
}

func (c *Client) applyEffort(ctx context.Context, p config.LLMProvider, body map[string]any) {
	c.mu.RLock()
	effort := c.effort
	c.mu.RUnlock()
	if c == nil || effort == nil || body == nil {
		return
	}
	if mode, ok := EffortModeFromContext(ctx); ok {
		ApplyEffort(body, p.API, effort.ResolveWithMode(ctx, p, mode))
		return
	}
	ApplyEffort(body, p.API, effort.ResolveFor(ctx, p))
}

func parseResponsesPayload(p config.LLMProvider, payload map[string]any) service.LLMResult {
	output, _ := payload["output"].([]any)
	var toolCalls []service.ToolCall
	var assistantText string
	for _, raw := range output {
		item, _ := raw.(map[string]any)
		if item == nil {
			continue
		}
		switch item["type"] {
		case "function_call":
			argsRaw := item["arguments"]
			var args map[string]any
			switch a := argsRaw.(type) {
			case string:
				_ = json.Unmarshal([]byte(a), &args)
			case map[string]any:
				args = a
			}
			if args == nil {
				args = map[string]any{}
			}
			callID, _ := item["call_id"].(string)
			if callID == "" {
				callID, _ = item["id"].(string)
			}
			if callID == "" {
				callID = idgen.New("call")
			}
			name, _ := item["name"].(string)
			toolCalls = append(toolCalls, service.ToolCall{CallID: callID, Name: name, Arguments: args})
		case "message":
			assistantText += extractMessageText(item)
		}
	}
	if assistantText == "" {
		if t, ok := payload["output_text"].(string); ok {
			assistantText = t
		}
	}
	toolCalls, assistantText = mergeXMLToolCalls(toolCalls, assistantText)
	usage, _ := payload["usage"].(map[string]any)
	status, _ := payload["status"].(string)
	return service.LLMResult{
		Text:       assistantText,
		ToolCalls:  toolCalls,
		Reasoning:  extractReasoningFromResponses(output),
		Model:      modelRef(p),
		Usage:      mapUsage(usage),
		ProviderID: p.ID,
		Status:     status,
	}
}

// extractMessageText pulls assistant-visible text from a Responses message item.
// Handles content as string or parts (output_text / text); ignores reasoning-only blocks.
func extractMessageText(item map[string]any) string {
	if item == nil {
		return ""
	}
	switch c := item["content"].(type) {
	case string:
		return c
	case []any:
		var out string
		for _, b := range c {
			block, _ := b.(map[string]any)
			if block == nil {
				continue
			}
			typ, _ := block["type"].(string)
			switch typ {
			case "output_text", "text", "summary_text":
				if t, ok := block["text"].(string); ok {
					out += t
				}
			case "":
				// Untyped part with text only.
				if t, ok := block["text"].(string); ok {
					out += t
				}
			}
		}
		return out
	}
	if t, ok := item["text"].(string); ok {
		return t
	}
	return ""
}

func (c *Client) postJSON(ctx context.Context, p config.LLMProvider, path string, body map[string]any) (map[string]any, error) {
	return c.postJSONStream(ctx, p, path, body, c.wantStream(), true)
}

func (c *Client) postJSONStream(ctx context.Context, p config.LLMProvider, path string, body map[string]any, stream bool, allowFallback bool) (map[string]any, error) {
	body["stream"] = stream
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, &Error{Provider: p.ID, Msg: "marshal body", Cause: err}
	}
	base := strings.TrimRight(p.BaseURL, "/") + "/"
	modelID, _ := body["model"].(string)
	logging.Info(ctx, "webchat.llm.request",
		"provider", p.ID,
		"url", base+path,
		"model", modelID,
		"stream", stream,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+path, bytes.NewReader(raw))
	if err != nil {
		return nil, &Error{Provider: p.ID, Msg: "build request", Cause: err, Transient: true}
	}
	if strings.TrimSpace(p.APIKey) != "" {
		if p.API == "messages" {
			req.Header.Set("x-api-key", p.APIKey)
			req.Header.Set("anthropic-version", "2023-06-01")
		} else {
			req.Header.Set("Authorization", "Bearer "+p.APIKey)
		}
	}
	req.Header.Set("Content-Type", "application/json")
	if stream {
		req.Header.Set("Accept", "text/event-stream, application/json")
	} else {
		req.Header.Set("Accept", "application/json")
	}
	// OpenRouter asks for these; harmless for other OpenAI-compatible proxies.
	req.Header.Set("HTTP-Referer", "https://buatpostingan.local")
	req.Header.Set("X-Title", "BuatPostingan")

	timeout := time.Duration(p.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	client := c.http
	if client.Timeout != timeout {
		client = &http.Client{Timeout: timeout}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, &Error{Provider: p.ID, Msg: "TIMEOUT/CONNECT", Cause: err, Transient: true}
	}
	defer resp.Body.Close()

	ctype := resp.Header.Get("Content-Type")
	br := bufio.NewReader(io.LimitReader(resp.Body, 8<<20))
	prefix, _ := br.Peek(64)
	snippet := truncateBody(prefix, 800)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		rest, _ := io.ReadAll(br)
		snippet = truncateBody(rest, 800)
		transient := resp.StatusCode == 0 || c.isTransient(resp.StatusCode)
		var retryAfter time.Duration
		if transient {
			if d, ok := parseRetryAfter(resp.Header.Get("Retry-After"), time.Now()); ok {
				retryAfter = d
			}
		}
		httpErr := &Error{
			Provider:   p.ID,
			Status:     resp.StatusCode,
			Transient:  transient,
			RetryAfter: retryAfter,
			Msg:        fmt.Sprintf("status=%d content-type=%s body=%s", resp.StatusCode, ctype, snippet),
		}
		if stream && allowFallback && isStreamUnsupported(httpErr) {
			logging.Warn(ctx, "webchat.llm.stream_fallback",
				"provider", p.ID, "path", path, "status", resp.StatusCode, "body", snippet)
			return c.postJSONStream(ctx, p, path, body, false, false)
		}
		return nil, httpErr
	}

	if looksLikeSSE(ctype, prefix) {
		payload, err := parseSSEToPayloadWithHooks(br, StreamHooksFromContext(ctx))
		if err != nil {
			// Codex CodexErr::Stream: incomplete / early close / mid-stream read → Transient.
			if errors.Is(err, ErrSSETransport) {
				return nil, &Error{
					Provider:  p.ID,
					Status:    resp.StatusCode,
					Cause:     err,
					Transient: true,
					Msg:       fmt.Sprintf("SSE_TRANSPORT status=%d content-type=%s err=%v", resp.StatusCode, ctype, err),
				}
			}
			return nil, &Error{
				Provider: p.ID,
				Status:   resp.StatusCode,
				Cause:    err,
				Msg:      fmt.Sprintf("BAD_BODY status=%d content-type=%s sse_parse=%v body=%s", resp.StatusCode, ctype, err, snippet),
			}
		}
		return payload, nil
	}

	respBody, _ := io.ReadAll(br)
	trimmed := bytes.TrimSpace(respBody)
	if len(trimmed) == 0 {
		emptyErr := &Error{
			Provider: p.ID,
			Status:   resp.StatusCode,
			Msg:      fmt.Sprintf("BAD_BODY status=%d content-type=%s empty body", resp.StatusCode, ctype),
		}
		if stream && allowFallback && isStreamUnsupported(emptyErr) {
			logging.Warn(ctx, "webchat.llm.stream_fallback",
				"provider", p.ID, "path", path, "status", resp.StatusCode, "reason", "empty_non_sse")
			return c.postJSONStream(ctx, p, path, body, false, false)
		}
		return nil, emptyErr
	}
	var payload map[string]any
	if err := json.Unmarshal(respBody, &payload); err != nil || payload == nil {
		badErr := &Error{
			Provider: p.ID,
			Status:   resp.StatusCode,
			Cause:    err,
			Msg:      fmt.Sprintf("BAD_BODY status=%d content-type=%s body=%s", resp.StatusCode, ctype, truncateBody(respBody, 800)),
		}
		if stream && allowFallback && isStreamUnsupported(badErr) {
			logging.Warn(ctx, "webchat.llm.stream_fallback",
				"provider", p.ID, "path", path, "status", resp.StatusCode, "body", truncateBody(respBody, 200))
			return c.postJSONStream(ctx, p, path, body, false, false)
		}
		return nil, badErr
	}
	// Some proxies return 200 + JSON error object when streaming is rejected.
	if stream && allowFallback && looksLikeJSONStreamReject(payload) {
		msg := truncateBody(respBody, 800)
		logging.Warn(ctx, "webchat.llm.stream_fallback",
			"provider", p.ID, "path", path, "status", resp.StatusCode, "body", msg)
		return c.postJSONStream(ctx, p, path, body, false, false)
	}
	return payload, nil
}

// isStreamUnsupported detects provider/proxy rejections that indicate streaming
// is not supported for this model/API — safe to retry once with stream=false.
// Skips auth/quota and unrelated 4xx (no stream-related signal in the body).
func isStreamUnsupported(err error) bool {
	var e *Error
	if !errors.As(err, &e) || e == nil {
		return false
	}
	switch e.Status {
	case 401, 402, 403:
		return false
	}
	msg := strings.ToLower(e.Msg)
	if e.Cause != nil {
		msg += " " + strings.ToLower(e.Cause.Error())
	}
	if !strings.Contains(msg, "stream") {
		return false
	}
	// Strong / common proxy phrases.
	strong := []string{
		"streaming not supported",
		"stream not supported",
		"does not support stream",
		"doesn't support stream",
		"streaming is not supported",
		"streaming unsupported",
		"stream unsupported",
		"streaming is disabled",
		"stream is disabled",
		"streaming not available",
		"stream not available",
		"stream=true is not",
		"cannot stream",
		"can't stream",
		"streaming is not enabled",
		"event-stream",
	}
	for _, s := range strong {
		if strings.Contains(msg, s) {
			return true
		}
	}
	// 4xx with "stream" + rejection language.
	if e.Status >= 400 && e.Status < 500 {
		for _, u := range []string{
			"not support", "unsupported", "not supported", "disabled",
			"not available", "unavailable", "not allowed", "not enabled",
			"invalid", "must be false", "only non-stream", "non-streaming",
		} {
			if strings.Contains(msg, u) {
				return true
			}
		}
	}
	return false
}

func looksLikeJSONStreamReject(payload map[string]any) bool {
	if payload == nil {
		return false
	}
	// OpenAI-shaped: {"error":{"message":"...stream..."}}
	if errObj, ok := payload["error"].(map[string]any); ok {
		parts := []string{
			fmt.Sprint(errObj["message"]),
			fmt.Sprint(errObj["code"]),
			fmt.Sprint(errObj["type"]),
			fmt.Sprint(payload["error"]),
		}
		blob := strings.ToLower(strings.Join(parts, " "))
		if strings.Contains(blob, "stream") {
			return isStreamUnsupported(&Error{Status: 400, Msg: blob})
		}
	}
	if msg, ok := payload["message"].(string); ok && strings.Contains(strings.ToLower(msg), "stream") {
		return isStreamUnsupported(&Error{Status: 400, Msg: msg})
	}
	return false
}

func truncateBody(b []byte, max int) string {
	s := strings.TrimSpace(string(b))
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > max {
		return s[:max]
	}
	return s
}

func looksLikeSSE(contentType string, body []byte) bool {
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "text/event-stream") {
		return true
	}
	prefix := string(body)
	if len(prefix) > 64 {
		prefix = prefix[:64]
	}
	prefix = strings.TrimLeft(prefix, " \t\r\n")
	return strings.HasPrefix(prefix, "event:") || strings.HasPrefix(prefix, "data:")
}

func looksLikeChatCompletion(payload map[string]any) bool {
	if payload == nil {
		return false
	}
	if obj, _ := payload["object"].(string); obj == "chat.completion" {
		return true
	}
	_, hasChoices := payload["choices"]
	_, hasOutput := payload["output"]
	return hasChoices && !hasOutput
}

func parseChatCompletionPayload(p config.LLMProvider, payload map[string]any) service.LLMResult {
	choices, _ := payload["choices"].([]any)
	var choice map[string]any
	if len(choices) > 0 {
		choice, _ = choices[0].(map[string]any)
	}
	msg, _ := choice["message"].(map[string]any)
	if msg == nil {
		msg = map[string]any{}
	}
	toolCalls := parseChatToolCalls(msg)
	text := extractChatContentText(msg["content"])
	toolCalls, text = mergeXMLToolCalls(toolCalls, text)
	usage, _ := payload["usage"].(map[string]any)
	status, _ := choice["finish_reason"].(string)
	return service.LLMResult{
		Text:       text,
		ToolCalls:  toolCalls,
		Reasoning:  extractReasoningFromChat(msg),
		Model:      modelRef(p),
		Usage:      mapUsage(usage),
		ProviderID: p.ID,
		Status:     status,
	}
}

func extractChatContentText(content any) string {
	switch c := content.(type) {
	case string:
		return c
	case []any:
		var out string
		for _, raw := range c {
			part, _ := raw.(map[string]any)
			if part == nil {
				continue
			}
			typ, _ := part["type"].(string)
			if typ == "text" || typ == "output_text" || typ == "" {
				if t, ok := part["text"].(string); ok {
					out += t
				}
			}
		}
		return out
	default:
		return ""
	}
}

func (c *Client) isTransient(status int) bool {
	_, ok := c.retry[status]
	return ok
}

func modelRef(p config.LLMProvider) service.ModelRef {
	return service.ModelRef{Provider: p.ID, ID: p.Model, API: p.API}
}

func parseChatToolCalls(msg map[string]any) []service.ToolCall {
	raw, _ := msg["tool_calls"].([]any)
	if len(raw) == 0 {
		return nil
	}
	out := make([]service.ToolCall, 0, len(raw))
	for _, r := range raw {
		tc, _ := r.(map[string]any)
		if tc == nil {
			continue
		}
		fn, _ := tc["function"].(map[string]any)
		id, _ := tc["id"].(string)
		if id == "" {
			id = idgen.New("call")
		}
		name, _ := fn["name"].(string)
		argsStr, _ := fn["arguments"].(string)
		var args map[string]any
		_ = json.Unmarshal([]byte(argsStr), &args)
		if args == nil {
			args = map[string]any{}
		}
		out = append(out, service.ToolCall{CallID: id, Name: name, Arguments: args})
	}
	return out
}

func mapUsage(usage map[string]any) service.TokenUsage {
	if usage == nil {
		return service.TokenUsage{}
	}
	u := service.TokenUsage{
		InputTokens:  asInt(usage["prompt_tokens"], usage["input_tokens"]),
		OutputTokens: asInt(usage["completion_tokens"], usage["output_tokens"]),
	}
	if details, ok := usage["prompt_tokens_details"].(map[string]any); ok {
		u.CachedInputTokens = asInt(details["cached_tokens"])
		u.CacheWriteTokens = asInt(details["cache_write_tokens"])
	}
	if details, ok := usage["input_tokens_details"].(map[string]any); ok {
		if u.CachedInputTokens == 0 {
			u.CachedInputTokens = asInt(details["cached_tokens"])
		}
		if u.CacheWriteTokens == 0 {
			u.CacheWriteTokens = asInt(details["cache_write_tokens"])
		}
	}
	if details, ok := usage["output_tokens_details"].(map[string]any); ok {
		u.ReasoningOutputTokens = asInt(details["reasoning_tokens"])
	}
	return u
}

func asInt(vals ...any) int {
	for _, v := range vals {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		case int64:
			return int(n)
		case json.Number:
			i, _ := n.Int64()
			return int(i)
		}
	}
	return 0
}

func extractReasoningFromChat(msg map[string]any) string {
	for _, key := range []string{"reasoning_content", "reasoning", "thinking"} {
		val := msg[key]
		if s, ok := val.(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func extractReasoningFromResponses(output []any) string {
	var parts []string
	for _, raw := range output {
		item, _ := raw.(map[string]any)
		if item == nil || item["type"] != "reasoning" {
			continue
		}
		for _, key := range []string{"summary", "content"} {
			blocks, _ := item[key].([]any)
			for _, b := range blocks {
				block, _ := b.(map[string]any)
				if block == nil {
					continue
				}
				if t, ok := block["text"].(string); ok && strings.TrimSpace(t) != "" {
					parts = append(parts, strings.TrimSpace(t))
				}
			}
		}
		if t, ok := item["text"].(string); ok && strings.TrimSpace(t) != "" {
			parts = append(parts, strings.TrimSpace(t))
		}
	}
	return strings.Join(parts, "\n\n")
}

func normalizeChatTools(tools []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		fn, _ := tool["function"].(map[string]any)
		if fn == nil {
			fn = tool
		}
		params, _ := fn["parameters"].(map[string]any)
		if params == nil {
			params = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		out = append(out, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        fn["name"],
				"description": fn["description"],
				"parameters":  normalizeParameters(params),
				"strict":      false,
			},
		})
	}
	return out
}

func toResponsesRequest(p config.LLMProvider, messages []map[string]any, tools []map[string]any, stream bool) map[string]any {
	var instructions []string
	var input []map[string]any
	for _, msg := range messages {
		role, _ := msg["role"].(string)
		switch role {
		case "system", "developer":
			if content, ok := msg["content"].(string); ok && content != "" {
				instructions = append(instructions, content)
			}
		case "user":
			input = append(input, map[string]any{"role": "user", "content": toResponsesUserContent(msg["content"])})
		case "assistant":
			if tcs, ok := msg["tool_calls"].([]any); ok && len(tcs) > 0 {
				for _, raw := range tcs {
					tc, _ := raw.(map[string]any)
					fn, _ := tc["function"].(map[string]any)
					input = append(input, map[string]any{
						"type":      "function_call",
						"call_id":   tc["id"],
						"name":      fn["name"],
						"arguments": fn["arguments"],
					})
				}
			} else if msg["content"] != nil {
				input = append(input, map[string]any{"role": "assistant", "content": fmt.Sprint(msg["content"])})
			}
		case "tool":
			input = append(input, map[string]any{
				"type":    "function_call_output",
				"call_id": msg["tool_call_id"],
				"output":  msg["content"],
			})
		}
	}
	body := map[string]any{
		"model":             p.Model,
		"input":             input,
		"tool_choice":       "auto",
		"max_output_tokens": p.MaxOutputTokens,
		"stream":            stream,
	}
	if len(instructions) > 0 {
		body["instructions"] = strings.Join(instructions, "\n\n")
	}
	if len(tools) > 0 {
		body["tools"] = toResponsesTools(tools)
	}
	return body
}

// toResponsesUserContent maps chat-completions content (string or multimodal
// parts) into Responses API content (string or input_text / input_image parts).
func toResponsesUserContent(content any) any {
	switch c := content.(type) {
	case nil:
		return ""
	case string:
		return c
	case []map[string]any:
		parts := make([]any, len(c))
		for i, p := range c {
			parts[i] = p
		}
		return toResponsesContentParts(parts)
	case []any:
		return toResponsesContentParts(c)
	default:
		return fmt.Sprint(c)
	}
}

func toResponsesContentParts(parts []any) []map[string]any {
	out := make([]map[string]any, 0, len(parts))
	for _, raw := range parts {
		p, ok := raw.(map[string]any)
		if !ok || p == nil {
			continue
		}
		switch p["type"] {
		case "text", "input_text":
			text := p["text"]
			out = append(out, map[string]any{"type": "input_text", "text": text})
		case "image_url", "input_image":
			url := extractImageURL(p)
			if url == "" {
				continue
			}
			part := map[string]any{"type": "input_image", "image_url": url}
			if detail, ok := p["detail"]; ok {
				part["detail"] = detail
			}
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return []map[string]any{{"type": "input_text", "text": ""}}
	}
	return out
}

func extractImageURL(p map[string]any) string {
	switch u := p["image_url"].(type) {
	case string:
		return strings.TrimSpace(u)
	case map[string]any:
		if s, ok := u["url"].(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func toResponsesTools(tools []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		fn, _ := tool["function"].(map[string]any)
		if fn == nil {
			fn = tool
		}
		params, _ := fn["parameters"].(map[string]any)
		if params == nil {
			params = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		out = append(out, map[string]any{
			"type":        "function",
			"name":        fn["name"],
			"description": fn["description"],
			"parameters":  normalizeParameters(params),
		})
	}
	return out
}

func normalizeParameters(parameters map[string]any) map[string]any {
	props, _ := parameters["properties"].(map[string]any)
	if props == nil {
		props = map[string]any{}
	}
	required, _ := parameters["required"].([]any)
	reqSet := map[string]struct{}{}
	for _, r := range required {
		if s, ok := r.(string); ok {
			reqSet[s] = struct{}{}
		}
	}
	allKeys := make([]string, 0, len(props))
	normalized := make(map[string]any, len(props))
	for key, schema := range props {
		allKeys = append(allKeys, key)
		sm, ok := schema.(map[string]any)
		if !ok {
			normalized[key] = schema
			continue
		}
		if _, req := reqSet[key]; !req {
			if t, ok := sm["type"].(string); ok {
				sm["type"] = tolerantOptionalTypes([]string{t})
			}
		}
		normalized[key] = sm
	}
	out := map[string]any{
		"type":                 "object",
		"properties":           normalized,
		"required":             allKeys,
		"additionalProperties": false,
	}
	return out
}

func tolerantOptionalTypes(types []string) []any {
	seen := map[string]struct{}{}
	var out []any
	add := func(t string) {
		if _, ok := seen[t]; ok {
			return
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	for _, t := range types {
		add(t)
		if t == "integer" || t == "number" || t == "boolean" {
			add("string")
		}
	}
	add("null")
	return out
}
