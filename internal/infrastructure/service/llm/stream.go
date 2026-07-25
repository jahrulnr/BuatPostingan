package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"buatpostingan/internal/pkg/logging"
)

// ErrSSETransport marks incomplete / mid-stream SSE failures (Codex CodexErr::Stream).
// Router treats these as Transient and retries within MaxAttempts / TotalAttemptBudget.
var ErrSSETransport = errors.New("sse transport")

// parseSSEToPayload consumes an OpenAI-compatible text/event-stream body and
// assembles a non-stream JSON-shaped payload (Responses or chat.completion)
// that the existing parsers understand.
func parseSSEToPayload(r io.Reader) (map[string]any, error) {
	return parseSSEToPayloadWithHooks(r, nil)
}

// parseSSEToPayloadWithHooks is like parseSSEToPayload but invokes hooks on text deltas.
func parseSSEToPayloadWithHooks(r io.Reader, hooks *StreamHooks) (map[string]any, error) {
	sc := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 2<<20) // large data: lines (tool args / long deltas)

	var (
		eventName string
		dataLines []string
		ass       = sseAssembler{hooks: hooks}
	)
	flush := func() error {
		if len(dataLines) == 0 {
			eventName = ""
			return nil
		}
		data := strings.Join(dataLines, "\n")
		dataLines = nil
		name := eventName
		eventName = ""
		if data == "[DONE]" {
			return nil
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(data), &obj); err != nil || obj == nil {
			// Skip non-JSON data lines (some proxies emit status comments as data).
			return nil
		}
		ass.feed(name, obj)
		return nil
	}

	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			if err := flush(); err != nil {
				return nil, err
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "event:") {
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			continue
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("%w: sse read: %w", ErrSSETransport, err)
	}
	return ass.payload()
}

type sseAssembler struct {
	gotResponses bool
	gotChat      bool
	hooks        *StreamHooks

	output           []map[string]any
	textDeltas       strings.Builder
	textDone         string
	reasoningDelta   strings.Builder
	reasoningDone    string
	usage            map[string]any
	failedMsg        string
	status           string
	sawCompleted     bool
	sawIncomplete    bool
	incompleteReason string

	chatContent   strings.Builder
	chatReasoning strings.Builder
	chatTools     map[int]*sseChatTool
	finishReason  string
}

func (a *sseAssembler) emitTextDelta(delta string) {
	if a == nil || delta == "" || a.hooks == nil || a.hooks.OnTextDelta == nil {
		return
	}
	a.hooks.OnTextDelta(delta)
}

type sseChatTool struct {
	id, name, args string
}

func (a *sseAssembler) feed(event string, obj map[string]any) {
	event = strings.TrimSpace(event)
	switch strings.ToLower(event) {
	case "error":
		if errObj, ok := obj["error"].(map[string]any); ok && errObj != nil {
			if msg, ok := errObj["message"].(string); ok && msg != "" {
				a.failedMsg = msg
			}
		}
		if a.failedMsg == "" {
			a.failedMsg = "sse error event"
		}
		return
	case "ping", "keepalive", "omniroute-keepalive":
		return
	}

	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	logging.Info(context.Background(), "webchat.llm.sse.feed",
		"event", event,
		"type", obj["type"],
		"object", obj["object"],
		"keys", strings.Join(keys, ","),
	)
	if objType, _ := obj["object"].(string); objType == "chat.completion.chunk" {
		a.feedChat(obj)
		return
	}
	if _, hasChoices := obj["choices"]; hasChoices && obj["object"] == nil {
		// Some proxies omit object on chunks.
		if _, hasType := obj["type"]; !hasType {
			a.feedChat(obj)
			return
		}
	}

	typ, _ := obj["type"].(string)
	if typ == "" {
		typ = event
	}
	if strings.HasPrefix(typ, "response.") {
		a.feedResponses(typ, obj)
		return
	}
	// Bare chat chunk without object field but with delta.
	if choices, _ := obj["choices"].([]any); len(choices) > 0 {
		if ch, _ := choices[0].(map[string]any); ch != nil {
			if _, ok := ch["delta"]; ok {
				a.feedChat(obj)
			}
		}
	}
}

func (a *sseAssembler) feedResponses(typ string, obj map[string]any) {
	a.gotResponses = true
	switch typ {
	case "response.output_item.done":
		if item, ok := obj["item"].(map[string]any); ok && item != nil {
			a.output = append(a.output, item)
		}
	case "response.output_text.delta":
		if d, ok := obj["delta"].(string); ok {
			a.textDeltas.WriteString(d)
			a.emitTextDelta(d)
		}
	case "response.output_text.done":
		if t, ok := obj["text"].(string); ok {
			a.textDone = t
		}
	case "response.function_call_arguments.delta":
		// Streaming tool-call args fragment. Final assembled arguments arrive in
		// response.output_item.done (item.arguments), so deltas are informational.
	case "response.function_call_arguments.done":
		// No-op: final args already captured via output_item.done.
	case "response.reasoning_summary_text.delta":
		if d, ok := obj["delta"].(string); ok {
			a.reasoningDelta.WriteString(d)
		}
	case "response.reasoning_summary_text.done":
		if t, ok := obj["text"].(string); ok {
			a.reasoningDone = t
		}
	case "response.completed":
		a.sawCompleted = true
		a.applyTerminalResponse(obj, "completed")
	case "response.incomplete":
		// Codex: response.incomplete → ApiError::Stream (retryable), not a success payload.
		a.sawIncomplete = true
		a.incompleteReason = "unknown"
		if resp, ok := obj["response"].(map[string]any); ok && resp != nil {
			if details, ok := resp["incomplete_details"].(map[string]any); ok {
				if r, ok := details["reason"].(string); ok && strings.TrimSpace(r) != "" {
					a.incompleteReason = strings.TrimSpace(r)
				}
			}
			a.applyTerminalResponse(obj, "incomplete")
		}
	case "response.failed":
		if resp, ok := obj["response"].(map[string]any); ok {
			if errObj, ok := resp["error"].(map[string]any); ok {
				if msg, ok := errObj["message"].(string); ok {
					a.failedMsg = msg
				}
			}
		}
		if a.failedMsg == "" {
			a.failedMsg = "response.failed"
		}
	}
}

func (a *sseAssembler) applyTerminalResponse(obj map[string]any, defaultStatus string) {
	resp, ok := obj["response"].(map[string]any)
	if !ok || resp == nil {
		a.status = defaultStatus
		return
	}
	if s, ok := resp["status"].(string); ok && s != "" {
		a.status = s
	} else {
		a.status = defaultStatus
	}
	if usage, ok := resp["usage"].(map[string]any); ok {
		a.usage = usage
	}
	if out, ok := resp["output"].([]any); ok && len(out) > 0 {
		a.output = a.output[:0]
		for _, raw := range out {
			if m, ok := raw.(map[string]any); ok {
				a.output = append(a.output, m)
			}
		}
	}
	if errObj, ok := resp["error"].(map[string]any); ok && errObj != nil {
		if msg, ok := errObj["message"].(string); ok && msg != "" {
			a.failedMsg = msg
		}
	}
}

func (a *sseAssembler) feedChat(obj map[string]any) {
	a.gotChat = true
	if usage, ok := obj["usage"].(map[string]any); ok {
		a.usage = usage
	}
	choices, _ := obj["choices"].([]any)
	if len(choices) == 0 {
		return
	}
	ch, _ := choices[0].(map[string]any)
	if ch == nil {
		return
	}
	delta, _ := ch["delta"].(map[string]any)
	if delta == nil {
		// Non-delta final message (rare).
		if msg, ok := ch["message"].(map[string]any); ok {
			if c, ok := msg["content"].(string); ok {
				a.chatContent.WriteString(c)
			}
		}
		return
	}
	if c, ok := delta["content"].(string); ok {
		a.chatContent.WriteString(c)
		a.emitTextDelta(c)
	}
	for _, key := range []string{"reasoning", "reasoning_content", "thinking"} {
		if s, ok := delta[key].(string); ok {
			a.chatReasoning.WriteString(s)
		}
	}
	if fr, ok := ch["finish_reason"].(string); ok && fr != "" {
		a.finishReason = fr
	}
	rawTCs, _ := delta["tool_calls"].([]any)
	if len(rawTCs) == 0 {
		return
	}
	if a.chatTools == nil {
		a.chatTools = map[int]*sseChatTool{}
	}
	for _, raw := range rawTCs {
		tc, _ := raw.(map[string]any)
		if tc == nil {
			continue
		}
		idx := asInt(tc["index"])
		slot := a.chatTools[idx]
		if slot == nil {
			slot = &sseChatTool{}
			a.chatTools[idx] = slot
		}
		if id, ok := tc["id"].(string); ok && id != "" {
			slot.id = id
		}
		fn, _ := tc["function"].(map[string]any)
		if fn != nil {
			if name, ok := fn["name"].(string); ok && name != "" {
				slot.name = name
			}
			if args, ok := fn["arguments"].(string); ok {
				slot.args += args
			}
		}
	}
}

func (a *sseAssembler) payload() (map[string]any, error) {
	if a.failedMsg != "" {
		return nil, fmt.Errorf("sse response failed: %s", a.failedMsg)
	}
	if a.gotResponses {
		// Codex: stream closed before response.completed → CodexErr::Stream (retryable).
		if a.sawIncomplete {
			reason := a.incompleteReason
			if reason == "" {
				reason = "unknown"
			}
			return nil, fmt.Errorf("%w: Incomplete response returned, reason: %s", ErrSSETransport, reason)
		}
		if !a.sawCompleted {
			return nil, fmt.Errorf("%w: closed before response.completed", ErrSSETransport)
		}
		return a.responsesPayload(), nil
	}
	if a.gotChat {
		return a.chatPayload(), nil
	}
	return nil, fmt.Errorf("sse stream produced no recognizable events")
}

func (a *sseAssembler) responsesPayload() map[string]any {
	text := a.textDone
	if text == "" {
		text = a.textDeltas.String()
	}
	output := make([]any, 0, len(a.output)+1)
	hasReasoningItem := false
	for _, item := range a.output {
		if item["type"] == "reasoning" {
			hasReasoningItem = true
		}
		output = append(output, item)
	}
	reasoning := a.reasoningDone
	if reasoning == "" {
		reasoning = a.reasoningDelta.String()
	}
	if reasoning != "" && !hasReasoningItem {
		output = append(output, map[string]any{
			"type": "reasoning",
			"summary": []any{
				map[string]any{"type": "summary_text", "text": reasoning},
			},
		})
	}
	// If deltas produced text but no message item, synthesize one.
	hasMessage := false
	for _, item := range a.output {
		if item["type"] == "message" {
			hasMessage = true
			break
		}
	}
	if text != "" && !hasMessage {
		output = append([]any{
			map[string]any{
				"type": "message",
				"role": "assistant",
				"content": []any{
					map[string]any{"type": "output_text", "text": text},
				},
			},
		}, output...)
	}
	status := a.status
	if status == "" {
		status = "completed"
	}
	if a.failedMsg != "" {
		status = "failed"
	}
	return map[string]any{
		"object":      "response",
		"status":      status,
		"output":      output,
		"output_text": text,
		"usage":       a.usage,
	}
}

func (a *sseAssembler) chatPayload() map[string]any {
	msg := map[string]any{
		"role":    "assistant",
		"content": a.chatContent.String(),
	}
	if a.chatReasoning.Len() > 0 {
		msg["reasoning"] = a.chatReasoning.String()
	}
	if len(a.chatTools) > 0 {
		max := -1
		for i := range a.chatTools {
			if i > max {
				max = i
			}
		}
		tcs := make([]any, 0, max+1)
		for i := 0; i <= max; i++ {
			slot := a.chatTools[i]
			if slot == nil {
				continue
			}
			tcs = append(tcs, map[string]any{
				"id":   slot.id,
				"type": "function",
				"function": map[string]any{
					"name":      slot.name,
					"arguments": slot.args,
				},
			})
		}
		if len(tcs) > 0 {
			msg["tool_calls"] = tcs
		}
	}
	choice := map[string]any{"message": msg}
	if a.finishReason != "" {
		choice["finish_reason"] = a.finishReason
	}
	return map[string]any{
		"object": "chat.completion",
		"choices": []any{
			choice,
		},
		"usage": a.usage,
	}
}
