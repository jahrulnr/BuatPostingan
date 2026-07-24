// Package worker runs the in-process TurnWorker (agent loop off HTTP).
package worker

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"
	"unicode/utf8"

	"buatpostingan/internal/config"
	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/enum"
	"buatpostingan/internal/domain/repository"
	"buatpostingan/internal/domain/service"
	"buatpostingan/internal/domain/valueobject"
	"buatpostingan/internal/pkg/idgen"
)

// Worker implements service.TurnWorker via goroutines (no external queue).
type Worker struct {
	cfg       config.Config
	store     repository.ThreadStore
	locks     repository.ThreadLock
	interrupt repository.InterruptFlag
	tools     service.ToolRegistry
	docs      service.DocsIndex
	llm       service.LLMRouter
}

var _ service.TurnWorker = (*Worker)(nil)

// Deps wires ports into the worker.
type Deps struct {
	Config    config.Config
	Store     repository.ThreadStore
	Locks     repository.ThreadLock
	Interrupt repository.InterruptFlag
	Tools     service.ToolRegistry
	Docs      service.DocsIndex
	LLM       service.LLMRouter
}

func New(deps Deps) *Worker {
	return &Worker{
		cfg:       deps.Config,
		store:     deps.Store,
		locks:     deps.Locks,
		interrupt: deps.Interrupt,
		tools:     deps.Tools,
		docs:      deps.Docs,
		llm:       deps.LLM,
	}
}

func (w *Worker) Enqueue(ctx context.Context, job service.TurnJob) error {
	_ = ctx
	timeout := time.Duration(w.cfg.TurnJobTimeoutSec) * time.Second
	if timeout < 30*time.Second {
		timeout = 30 * time.Second
	}
	go func() {
		jobCtx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		w.process(jobCtx, job)
	}()
	return nil
}

func (w *Worker) process(ctx context.Context, job service.TurnJob) {
	log.Printf("webchat.turn_start thread=%s turn=%s stub=%v resume=%v",
		job.ThreadID, job.TurnID, w.cfg.LLMStub, job.IsRetry)

	defer func() {
		_ = w.store.ClearActiveTurn(context.Background(), job.ThreadID)
		_ = w.locks.Release(context.Background(), job.ThreadID, job.LockToken)
	}()

	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("webchat.turn_panic thread=%s turn=%s: %v", job.ThreadID, job.TurnID, rec)
			_, _ = w.append(ctx, job, enum.ItemTurnFailed, map[string]any{
				"error": map[string]any{
					"code":    "job_error",
					"message": "panic in turn worker",
				},
			})
		}
	}()

	if err := w.run(ctx, job); err != nil {
		msg := err.Error()
		if utf8.RuneCountInString(msg) > 1000 {
			msg = string([]rune(msg)[:1000])
		}
		log.Printf("webchat.turn_failed thread=%s turn=%s: %s", job.ThreadID, job.TurnID, msg)
		_, _ = w.append(ctx, job, enum.ItemTurnFailed, map[string]any{
			"error": map[string]any{
				"code":    "job_error",
				"message": msg,
			},
		})
	} else {
		log.Printf("webchat.turn_completed thread=%s turn=%s", job.ThreadID, job.TurnID)
	}
}

func (w *Worker) run(ctx context.Context, job service.TurnJob) error {
	if job.IsRetry {
		text, ok := w.findTurnUserText(ctx, job)
		if !ok {
			_, err := w.append(ctx, job, enum.ItemTurnFailed, map[string]any{
				"error": map[string]any{"code": "not_found", "message": "turn missing user_message"},
			})
			return err
		}
		job.Message = text
		if _, err := w.append(ctx, job, enum.ItemTurnResumed, nil); err != nil {
			return err
		}
	} else {
		if _, err := w.append(ctx, job, enum.ItemUserMessage, map[string]any{
			"text":               job.Message,
			"admin_user_id":      job.AdminUserID,
			"admin_display_name": displayName(job.AdminName),
		}); err != nil {
			return err
		}
		if _, err := w.append(ctx, job, enum.ItemTurnStarted, nil); err != nil {
			return err
		}
	}

	if err := w.markActiveTurn(ctx, job); err != nil {
		return err
	}

	if w.cfg.LLMStub {
		if err := w.runStub(ctx, job); err != nil {
			return err
		}
	} else {
		if err := w.runAgent(ctx, job); err != nil {
			return err
		}
	}

	if w.turnCompleted(ctx, job) {
		w.maybeAutoTitle(ctx, job)
	}
	return nil
}

func (w *Worker) runStub(ctx context.Context, job service.TurnJob) error {
	if _, err := w.append(ctx, job, enum.ItemAgentMessage, map[string]any{
		"text": "(stub) received: " + job.Message,
	}); err != nil {
		return err
	}
	_, err := w.append(ctx, job, enum.ItemTurnCompleted, map[string]any{
		"usage": emptyUsage(),
	})
	return err
}

func (w *Worker) runAgent(ctx context.Context, job service.TurnJob) error {
	if w.llm == nil || w.tools == nil {
		return errf("llm/tools not configured")
	}
	schemas, err := w.tools.Schemas(ctx)
	if err != nil {
		return err
	}
	toolNames := make([]string, 0, len(schemas))
	for _, t := range schemas {
		fn, _ := t["function"].(map[string]any)
		if name, _ := fn["name"].(string); name != "" {
			toolNames = append(toolNames, name)
		}
	}

	snap, err := w.store.GetThread(ctx, job.ThreadID, 0)
	if err != nil {
		return err
	}
	messages := buildMessages(snap.Items)
	docCount := 0
	if w.docs != nil {
		if gate, gerr := w.docs.Gate(ctx); gerr == nil {
			docCount = gate.DocumentCount
		}
	}
	messages, err = injectPrompts(w.cfg.PromptsRoot, messages, promptVars{
		AdminDisplayName:     displayName(job.AdminName),
		AdminUserID:          job.AdminUserID,
		AvailableTools:       strings.Join(toolNames, ", "),
		IndexedDocumentCount: docCount,
	})
	if err != nil {
		return err
	}

	maxRounds := w.cfg.MaxToolRounds
	if maxRounds < 1 {
		maxRounds = 8
	}
	var pinned string
	lastUsage := emptyUsage()
	var lastModel map[string]any
	lastToolOnly := false
	rounds := 0
	var prevToolFingerprint string
	identicalToolRounds := 0

	for rounds < maxRounds {
		rounds++
		if interrupted, _ := w.interrupt.IsRequested(ctx, job.ThreadID, job.TurnID); interrupted {
			_ = w.interrupt.Clear(ctx, job.ThreadID, job.TurnID)
			_, err := w.append(ctx, job, enum.ItemTurnFailed, map[string]any{
				"error": map[string]any{"code": "interrupted", "message": "Stopped by user"},
			})
			return err
		}

		resp, err := w.llm.Chat(ctx, messages, schemas, pinned)
		if err != nil {
			return err
		}
		if resp.ProviderID != "" {
			pinned = resp.ProviderID
		}
		lastUsage = usageMap(resp.Usage)
		lastModel = modelMetadata(resp, "response")

		if strings.TrimSpace(resp.Reasoning) != "" {
			text := resp.Reasoning
			if utf8.RuneCountInString(text) > 12000 {
				text = string([]rune(text)[:12000])
			}
			log.Printf("webchat.reasoning thread=%s turn=%s round=%d chars=%d",
				job.ThreadID, job.TurnID, rounds, utf8.RuneCountInString(text))
			if _, err := w.append(ctx, job, enum.ItemReasoning, map[string]any{
				"text":  text,
				"model": modelMetadata(resp, "reasoning"),
			}); err != nil {
				return err
			}
		}

		if len(resp.ToolCalls) > 0 {
			fp := toolCallsFingerprint(resp.ToolCalls)
			if fp != "" && fp == prevToolFingerprint {
				identicalToolRounds++
			} else {
				identicalToolRounds = 0
			}
			prevToolFingerprint = fp

			// Normalize call IDs once so function_call and function_call_output match.
			for i := range resp.ToolCalls {
				if resp.ToolCalls[i].CallID == "" {
					resp.ToolCalls[i].CallID = idgen.New("call")
				}
			}
			messages = append(messages, assistantToolMessage(resp.ToolCalls))
			lastToolOnly = true
			for _, tc := range resp.ToolCalls {
				callID := tc.CallID
				if _, err := w.append(ctx, job, enum.ItemToolCall, map[string]any{
					"call_id":   callID,
					"name":      tc.Name,
					"arguments": tc.Arguments,
					"model":     modelMetadata(resp, "planner"),
				}); err != nil {
					return err
				}
				envelope, execErr := w.tools.Execute(ctx, service.ToolCall{
					CallID:    callID,
					Name:      tc.Name,
					Arguments: tc.Arguments,
				})
				if execErr != nil {
					envelope = service.ToolEnvelope{
						OK:   false,
						Tool: tc.Name,
						Error: map[string]any{
							"code":    "tool_error",
							"message": execErr.Error(),
						},
					}
				}
				envMap := envelopeToMap(envelope)
				raw, _ := json.Marshal(envMap)
				log.Printf("webchat.tool thread=%s turn=%s round=%d tool=%s call_id=%s ok=%v args_bytes=%d result_bytes=%d",
					job.ThreadID, job.TurnID, rounds, tc.Name, callID, envelope.OK,
					len(mustJSON(tc.Arguments)), len(raw))
				if _, err := w.append(ctx, job, enum.ItemToolResult, map[string]any{
					"call_id":  callID,
					"envelope": envMap,
					"executor": "host_tool",
				}); err != nil {
					return err
				}
				messages = append(messages, map[string]any{
					"role":         "tool",
					"tool_call_id": callID,
					"content":      string(raw),
				})
			}
			if identicalToolRounds >= 1 {
				nudge := "You repeated the same tool call with identical arguments. The result is already in the conversation. Answer the user now, or call a different tool / different arguments (e.g. list_dir path=\"writing\")."
				messages = append(messages, map[string]any{"role": "system", "content": nudge})
				log.Printf("webchat.tool_dedupe thread=%s turn=%s round=%d fingerprint=%s",
					job.ThreadID, job.TurnID, rounds, fp)
			}
			if identicalToolRounds >= 2 {
				if _, err := w.append(ctx, job, enum.ItemAgentMessage, map[string]any{
					"text":   "Stopped: repeated identical tool calls. Please refine the question or try a different path/query.",
					"origin": "runtime",
				}); err != nil {
					return err
				}
				lastToolOnly = false
				break
			}
			continue
		}

		text := resp.Text
		if strings.TrimSpace(text) == "" {
			text = "(empty model response)"
		}
		if _, err := w.append(ctx, job, enum.ItemAgentMessage, map[string]any{
			"text":  text,
			"model": modelMetadata(resp, "response"),
		}); err != nil {
			return err
		}
		lastToolOnly = false
		break
	}

	if rounds >= maxRounds && lastToolOnly {
		if _, err := w.append(ctx, job, enum.ItemAgentMessage, map[string]any{
			"text":   "Stopped: max tool rounds reached. Please refine the question.",
			"origin": "runtime",
		}); err != nil {
			return err
		}
	}

	payload := map[string]any{"usage": lastUsage}
	if lastModel != nil {
		payload["model"] = lastModel
	}
	_, err = w.append(ctx, job, enum.ItemTurnCompleted, payload)
	return err
}

func (w *Worker) markActiveTurn(ctx context.Context, job service.TurnJob) error {
	prev, ok, err := w.store.ResolveConversation(ctx, job.ThreadID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	meta := prev
	if !ok {
		meta = entity.ConversationMeta{
			ThreadID:             job.ThreadID,
			TitleSource:          enum.TitlePending,
			Status:               enum.ConversationActive,
			CreatedByAdminUserID: job.AdminUserID,
		}
	}
	meta.Status = enum.ConversationActive
	meta.UpdatedAt = now
	meta.LastActivityAt = now
	turn := job.TurnID
	meta.ActiveTurnID = &turn
	initiator := job.AdminUserID
	meta.ActiveTurnInitiatorAdminID = &initiator
	if meta.FloorHolderAdminID == nil {
		meta.FloorHolderAdminID = &initiator
	}
	return w.store.AppendConversationMeta(ctx, meta)
}

func (w *Worker) maybeAutoTitle(ctx context.Context, job service.TurnJob) {
	prev, ok, err := w.store.ResolveConversation(ctx, job.ThreadID)
	if err != nil || !ok {
		return
	}
	if prev.TitleSource != enum.TitlePending && prev.TitleSource != "" {
		return
	}
	title, err := valueobject.NewTitle(truncateRunes(job.Message, 60))
	if err != nil {
		return
	}
	prev.Title = &title
	prev.TitleSource = enum.TitleAuto
	prev.UpdatedAt = time.Now().UTC()
	prev.LastActivityAt = prev.UpdatedAt
	_ = w.store.AppendConversationMeta(ctx, prev)
}

func (w *Worker) append(ctx context.Context, job service.TurnJob, typ enum.ItemType, payload map[string]any) (entity.TranscriptItem, error) {
	id, err := valueobject.NewItemID(idgen.ItemID())
	if err != nil {
		return entity.TranscriptItem{}, err
	}
	if payload == nil {
		payload = map[string]any{}
	}
	return w.store.AppendItem(ctx, job.ThreadID, entity.TranscriptItem{
		ID:       id,
		ThreadID: job.ThreadID,
		TurnID:   job.TurnID,
		Type:     typ,
		Payload:  payload,
		At:       time.Now().UTC(),
	})
}

func (w *Worker) findTurnUserText(ctx context.Context, job service.TurnJob) (string, bool) {
	snap, err := w.store.GetThread(ctx, job.ThreadID, 0)
	if err != nil {
		return "", false
	}
	for _, it := range snap.Items {
		if it.Type == enum.ItemUserMessage && it.TurnID == job.TurnID {
			text, _ := it.Payload["text"].(string)
			return text, true
		}
	}
	return "", false
}

func (w *Worker) turnCompleted(ctx context.Context, job service.TurnJob) bool {
	snap, err := w.store.GetThread(ctx, job.ThreadID, 0)
	if err != nil {
		return false
	}
	for i := len(snap.Items) - 1; i >= 0; i-- {
		it := snap.Items[i]
		if it.TurnID != job.TurnID {
			continue
		}
		switch it.Type {
		case enum.ItemTurnCompleted:
			return true
		case enum.ItemTurnFailed:
			return false
		}
	}
	return false
}

func buildMessages(items []entity.TranscriptItem) []map[string]any {
	var msgs []map[string]any
	for i, it := range items {
		if it.Type == enum.ItemContextCompact {
			text, _ := it.Payload["text"].(string)
			msgs = []map[string]any{{
				"role":    "system",
				"content": "Conversation summary:\n" + text,
			}}
			for _, prev := range items[:i] {
				if prev.TurnID != it.TurnID {
					continue
				}
				appendMessageFromItem(&msgs, prev)
			}
			continue
		}
		appendMessageFromItem(&msgs, it)
	}
	return msgs
}

func appendMessageFromItem(msgs *[]map[string]any, it entity.TranscriptItem) {
	switch it.Type {
	case enum.ItemUserMessage:
		text, _ := it.Payload["text"].(string)
		*msgs = append(*msgs, map[string]any{"role": "user", "content": text})
	case enum.ItemAgentMessage:
		text, _ := it.Payload["text"].(string)
		*msgs = append(*msgs, map[string]any{"role": "assistant", "content": text})
	case enum.ItemToolCall:
		callID, _ := it.Payload["call_id"].(string)
		name, _ := it.Payload["name"].(string)
		args := it.Payload["arguments"]
		raw, _ := json.Marshal(args)
		*msgs = append(*msgs, map[string]any{
			"role":    "assistant",
			"content": nil,
			"tool_calls": []any{
				map[string]any{
					"id":   callID,
					"type": "function",
					"function": map[string]any{
						"name":      name,
						"arguments": string(raw),
					},
				},
			},
		})
	case enum.ItemToolResult:
		callID, _ := it.Payload["call_id"].(string)
		raw, _ := json.Marshal(it.Payload["envelope"])
		*msgs = append(*msgs, map[string]any{
			"role":         "tool",
			"tool_call_id": callID,
			"content":      string(raw),
		})
	}
}

func assistantToolMessage(calls []service.ToolCall) map[string]any {
	tcs := make([]any, 0, len(calls))
	for _, tc := range calls {
		raw, _ := json.Marshal(tc.Arguments)
		id := tc.CallID
		if id == "" {
			id = idgen.New("call")
		}
		tcs = append(tcs, map[string]any{
			"id":   id,
			"type": "function",
			"function": map[string]any{
				"name":      tc.Name,
				"arguments": string(raw),
			},
		})
	}
	return map[string]any{
		"role":       "assistant",
		"content":    nil,
		"tool_calls": tcs,
	}
}

func toolCallsFingerprint(calls []service.ToolCall) string {
	parts := make([]string, 0, len(calls))
	for _, tc := range calls {
		parts = append(parts, tc.Name+"\x00"+string(mustJSON(tc.Arguments)))
	}
	return strings.Join(parts, "\x01")
}

func mustJSON(v any) []byte {
	raw, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return raw
}

func modelMetadata(resp service.LLMResult, role string) map[string]any {
	provider := resp.Model.Provider
	if provider == "" {
		provider = resp.ProviderID
	}
	if provider == "" {
		provider = "unknown"
	}
	id := resp.Model.ID
	if id == "" {
		id = "unknown"
	}
	api := resp.Model.API
	if api == "" {
		api = "unknown"
	}
	return map[string]any{
		"provider": provider,
		"id":       id,
		"api":      api,
		"role":     role,
	}
}

func usageMap(u service.TokenUsage) map[string]any {
	return map[string]any{
		"input_tokens":            u.InputTokens,
		"cached_input_tokens":     u.CachedInputTokens,
		"cache_write_tokens":      u.CacheWriteTokens,
		"output_tokens":           u.OutputTokens,
		"reasoning_output_tokens": u.ReasoningOutputTokens,
	}
}

func emptyUsage() map[string]any {
	return map[string]any{
		"input_tokens":            0,
		"cached_input_tokens":     0,
		"cache_write_tokens":      0,
		"output_tokens":           0,
		"reasoning_output_tokens": 0,
	}
}

func envelopeToMap(e service.ToolEnvelope) map[string]any {
	m := map[string]any{
		"ok":   e.OK,
		"tool": e.Tool,
		"data": e.Data,
	}
	if e.Error != nil {
		m["error"] = e.Error
	}
	if e.Meta != nil {
		m["meta"] = e.Meta
	}
	return m
}

func displayName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Admin"
	}
	return name
}

func truncateRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "New chat"
	}
	r := []rune(s)
	if len(r) > max {
		return string(r[:max])
	}
	return s
}

func errf(msg string) error { return &simpleErr{msg: msg} }

type simpleErr struct{ msg string }

func (e *simpleErr) Error() string { return e.msg }
