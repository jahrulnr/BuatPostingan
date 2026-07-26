package llm

import (
	"encoding/json"
	"fmt"
	"strings"

	"buatpostingan/internal/config"
	"buatpostingan/internal/domain/service"
	"buatpostingan/internal/pkg/idgen"
)

// toMessagesRequest maps the worker's canonical OpenAI-shaped conversation to
// Anthropic Messages without duplicating transport, retry, or routing.
func toMessagesRequest(p config.LLMProvider, messages []map[string]any, tools []map[string]any) map[string]any {
	var system []string
	out := make([]map[string]any, 0, len(messages))
	for _, msg := range messages {
		role, _ := msg["role"].(string)
		switch role {
		case "system", "developer":
			if text := extractChatContentText(msg["content"]); text != "" {
				system = append(system, text)
			}
		case "user":
			out = append(out, map[string]any{"role": "user", "content": toMessagesUserContent(msg["content"])})
		case "assistant":
			if content := toMessagesAssistantContent(msg); len(content) > 0 {
				out = append(out, map[string]any{"role": "assistant", "content": content})
			}
		case "tool":
			out = append(out, map[string]any{
				"role": "user",
				"content": []any{map[string]any{
					"type":        "tool_result",
					"tool_use_id": msg["tool_call_id"],
					"content":     fmt.Sprint(msg["content"]),
				}},
			})
		}
	}
	body := map[string]any{
		"model":      p.Model,
		"messages":   out,
		"max_tokens": p.MaxOutputTokens,
		"stream":     false,
	}
	if len(system) > 0 {
		body["system"] = strings.Join(system, "\n\n")
	}
	if len(tools) > 0 {
		body["tools"] = toMessagesTools(tools)
	}
	return body
}

func toMessagesAssistantContent(msg map[string]any) []any {
	content := make([]any, 0, 2)
	if text := extractChatContentText(msg["content"]); text != "" {
		content = append(content, map[string]any{"type": "text", "text": text})
	}
	rawCalls, _ := msg["tool_calls"].([]any)
	for _, raw := range rawCalls {
		call, _ := raw.(map[string]any)
		fn, _ := call["function"].(map[string]any)
		if fn == nil {
			continue
		}
		input := map[string]any{}
		switch args := fn["arguments"].(type) {
		case string:
			_ = json.Unmarshal([]byte(args), &input)
		case map[string]any:
			input = args
		}
		content = append(content, map[string]any{
			"type":  "tool_use",
			"id":    call["id"],
			"name":  fn["name"],
			"input": input,
		})
	}
	return content
}

func toMessagesUserContent(content any) any {
	parts, ok := content.([]any)
	if !ok {
		return content
	}
	out := make([]any, 0, len(parts))
	for _, raw := range parts {
		part, _ := raw.(map[string]any)
		switch part["type"] {
		case "text", "input_text":
			out = append(out, map[string]any{"type": "text", "text": part["text"]})
		case "image_url", "input_image":
			mediaType, data, ok := splitBase64DataURL(extractImageURL(part))
			if !ok {
				continue
			}
			out = append(out, map[string]any{
				"type": "image",
				"source": map[string]any{
					"type":       "base64",
					"media_type": mediaType,
					"data":       data,
				},
			})
		}
	}
	return out
}

func splitBase64DataURL(raw string) (mediaType, data string, ok bool) {
	if !strings.HasPrefix(raw, "data:") {
		return "", "", false
	}
	header, data, found := strings.Cut(strings.TrimPrefix(raw, "data:"), ",")
	if !found || !strings.HasSuffix(header, ";base64") || strings.TrimSpace(data) == "" {
		return "", "", false
	}
	mediaType = strings.TrimSuffix(header, ";base64")
	if !strings.HasPrefix(mediaType, "image/") {
		return "", "", false
	}
	return mediaType, data, true
}

func toMessagesTools(tools []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		fn, _ := tool["function"].(map[string]any)
		if fn == nil {
			fn = tool
		}
		schema, _ := fn["parameters"].(map[string]any)
		if schema == nil {
			schema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		out = append(out, map[string]any{
			"name":         fn["name"],
			"description":  fn["description"],
			"input_schema": normalizeParameters(schema),
		})
	}
	return out
}

func parseMessagesPayload(p config.LLMProvider, payload map[string]any) service.LLMResult {
	content, _ := payload["content"].([]any)
	var text string
	var reasoning string
	var calls []service.ToolCall
	for _, raw := range content {
		block, _ := raw.(map[string]any)
		switch block["type"] {
		case "text":
			text += fmt.Sprint(block["text"])
		case "thinking":
			reasoning += fmt.Sprint(block["thinking"])
		case "tool_use":
			input, _ := block["input"].(map[string]any)
			if input == nil {
				input = map[string]any{}
			}
			callID, _ := block["id"].(string)
			if callID == "" {
				callID = idgen.New("call")
			}
			name, _ := block["name"].(string)
			calls = append(calls, service.ToolCall{CallID: callID, Name: name, Arguments: input})
		}
	}
	calls, text = mergeXMLToolCalls(calls, text)
	usage, _ := payload["usage"].(map[string]any)
	status, _ := payload["stop_reason"].(string)
	return service.LLMResult{
		Text: text, ToolCalls: calls, Reasoning: reasoning,
		Model: modelRef(p), Usage: mapUsage(usage), ProviderID: p.ID, Status: status,
	}
}
