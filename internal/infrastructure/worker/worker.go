// Package worker runs the in-process TurnWorker (agent loop off HTTP).
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"buatpostingan/internal/config"
	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/enum"
	"buatpostingan/internal/domain/repository"
	"buatpostingan/internal/domain/service"
	"buatpostingan/internal/domain/valueobject"
	"buatpostingan/internal/infrastructure/service/llm"
	"buatpostingan/internal/infrastructure/service/tools"
	"buatpostingan/internal/pkg/idgen"
	"buatpostingan/internal/pkg/logging"
)

// Worker implements service.TurnWorker via goroutines (no external queue).
type Worker struct {
	cfg         config.Config
	store       repository.ThreadStore
	locks       repository.ThreadLock
	interrupt   repository.InterruptFlag
	tools       service.ToolRegistry
	docs        service.DocsIndex
	llm         service.LLMRouter
	attachments repository.AttachmentStore
	vision      visionGate
	hub         service.ThreadEventHub
}

var _ service.TurnWorker = (*Worker)(nil)

// Deps wires ports into the worker.
type Deps struct {
	Config      config.Config
	Store       repository.ThreadStore
	Locks       repository.ThreadLock
	Interrupt   repository.InterruptFlag
	Tools       service.ToolRegistry
	Docs        service.DocsIndex
	LLM         service.LLMRouter
	Attachments repository.AttachmentStore
	// Vision optional; when nil, pixels are allowed (tests / legacy).
	Vision visionGate
	// Hub optional; when set, LLM text deltas are published as ephemeral SSE.
	Hub service.ThreadEventHub
}

func New(deps Deps) *Worker {
	return &Worker{
		cfg:         deps.Config,
		store:       deps.Store,
		locks:       deps.Locks,
		interrupt:   deps.Interrupt,
		tools:       deps.Tools,
		docs:        deps.Docs,
		llm:         deps.LLM,
		attachments: deps.Attachments,
		vision:      deps.Vision,
		hub:         deps.Hub,
	}
}

// Reload updates runtime config (e.g. LLMStub after settings save).
func (w *Worker) Reload(cfg config.Config) {
	if w == nil {
		return
	}
	w.cfg = cfg
}

func (w *Worker) Enqueue(ctx context.Context, job service.TurnJob) error {
	if job.TraceID == "" {
		job.TraceID = logging.TraceID(ctx)
	}
	if job.TraceID == "" {
		job.TraceID = logging.TraceSystem
	}
	timeout := time.Duration(w.cfg.TurnJobTimeoutSec) * time.Second
	if timeout < 30*time.Second {
		timeout = 30 * time.Second
	}
	go func() {
		jobCtx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		jobCtx = logging.WithTraceID(jobCtx, job.TraceID)
		w.process(jobCtx, job)
	}()
	return nil
}

func (w *Worker) process(ctx context.Context, job service.TurnJob) {
	logging.Info(ctx, "webchat.turn_start",
		"thread", job.ThreadID.String(),
		"turn", job.TurnID.String(),
		"stub", w.cfg.LLMStub,
		"resume", job.IsRetry,
	)

	defer func() {
		_ = w.store.ClearActiveTurn(context.Background(), job.ThreadID)
		_ = w.locks.Release(context.Background(), job.ThreadID, job.LockToken)
	}()

	defer func() {
		if rec := recover(); rec != nil {
			logging.Error(ctx, "webchat.turn_panic", fmt.Errorf("%v", rec),
				"thread", job.ThreadID.String(),
				"turn", job.TurnID.String(),
			)
			_, _ = w.append(ctx, job, enum.ItemTurnFailed, map[string]any{
				"error": map[string]any{
					"code":    "job_error",
					"message": "panic in turn worker",
				},
				"trace_id": logging.TraceID(ctx),
			})
		}
	}()

	if err := w.run(ctx, job); err != nil {
		msg := err.Error()
		if utf8.RuneCountInString(msg) > 1000 {
			msg = string([]rune(msg)[:1000])
		}
		logging.Error(ctx, "webchat.turn_failed", err,
			"thread", job.ThreadID.String(),
			"turn", job.TurnID.String(),
		)
		_, _ = w.append(ctx, job, enum.ItemTurnFailed, map[string]any{
			"error": map[string]any{
				"code":    "job_error",
				"message": msg,
			},
			"trace_id": logging.TraceID(ctx),
		})
	} else {
		logging.Info(ctx, "webchat.turn_completed",
			"thread", job.ThreadID.String(),
			"turn", job.TurnID.String(),
		)
	}
}

func (w *Worker) run(ctx context.Context, job service.TurnJob) error {
	if job.IsRetry {
		text, ok := w.findTurnUserText(ctx, job)
		if !ok {
			_, err := w.append(ctx, job, enum.ItemTurnFailed, map[string]any{
				"error":    map[string]any{"code": "not_found", "message": "turn missing user_message"},
				"trace_id": logging.TraceID(ctx),
			})
			return err
		}
		job.Message = text
		if _, err := w.append(ctx, job, enum.ItemTurnResumed, nil); err != nil {
			return err
		}
	} else {
		payload := map[string]any{
			"text":               job.Message,
			"admin_user_id":      job.AdminUserID,
			"admin_display_name": displayName(job.AdminName),
		}
		if refs := w.attachmentRefs(ctx, job); len(refs) > 0 {
			payload["attachments"] = refs
		}
		if _, err := w.append(ctx, job, enum.ItemUserMessage, payload); err != nil {
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
	text := "(stub) received: " + job.Message
	if len(job.AttachmentIDs) > 0 {
		text += " · attachments=" + strings.Join(job.AttachmentIDs, ",")
	}
	if _, err := w.append(ctx, job, enum.ItemAgentMessage, map[string]any{
		"text": text,
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
	items, err := w.maybeCompact(ctx, job, snap.Items)
	if err != nil {
		return err
	}
	messages := w.buildMessages(ctx, job.ThreadID, items)
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
		Workspace:            job.Workspace,
	})
	if err != nil {
		return err
	}

	maxRounds := w.cfg.MaxToolRounds
	if maxRounds < 1 {
		maxRounds = 8
	}
	pinned := strings.TrimSpace(job.ProviderID)
	chatCtx := ctx
	if strings.TrimSpace(job.Effort) != "" {
		chatCtx = llm.WithEffortMode(ctx, job.Effort)
	}
	if strings.TrimSpace(job.Model) != "" {
		chatCtx = llm.WithModelOverride(chatCtx, job.Model)
	}
	lastUsage := emptyUsage()
	var lastModel map[string]any
	lastToolOnly := false
	rounds := 0
	var prevToolFingerprint string
	identicalToolRounds := 0
	emptyNudged := false

	for rounds < maxRounds {
		rounds++
		if interrupted, _ := w.interrupt.IsRequested(ctx, job.ThreadID, job.TurnID); interrupted {
			_ = w.interrupt.Clear(ctx, job.ThreadID, job.TurnID)
			_, err := w.append(ctx, job, enum.ItemTurnFailed, map[string]any{
				"error":    map[string]any{"code": "interrupted", "message": "Stopped by user"},
				"trace_id": logging.TraceID(ctx),
			})
			return err
		}

		draftItemID := idgen.ItemID()
		streamedText := false
		var streamedTextBuf strings.Builder
		var streamedTextMu sync.Mutex
		turnID := job.TurnID.String()
		threadID := job.ThreadID
		itemID := draftItemID
		roundCtx := llm.WithStreamHooks(chatCtx, &llm.StreamHooks{
			OnTextDelta: func(delta string) {
				if delta == "" {
					return
				}
				streamedTextMu.Lock()
				streamedText = true
				streamedTextBuf.WriteString(delta)
				streamedTextMu.Unlock()
				if w.hub != nil {
					w.hub.PublishEphemeral(threadID, "item.delta", map[string]any{
						"type":    "agent_message",
						"turn_id": turnID,
						"item_id": itemID,
						"field":   "text",
						"delta":   delta,
					})
				}
			},
		})

		resp, err := w.llm.Chat(roundCtx, messages, schemas, pinned)
		if err != nil {
			return err
		}
		streamedTextMu.Lock()
		streamed := streamedTextBuf.String()
		streamedTextMu.Unlock()
		resp = llm.RecoverXMLToolCalls(resp, streamed)
		logging.Warn(ctx, "webchat.llm.result",
			"round", rounds,
			"provider", resp.ProviderID,
			"textLen", len(resp.Text),
			"toolCalls", len(resp.ToolCalls),
			"streamedLen", len(streamed),
		)
		for i, tc := range resp.ToolCalls {
			keys := make([]string, 0, len(tc.Arguments))
			for k := range tc.Arguments {
				keys = append(keys, k)
			}
			logging.Warn(ctx, "webchat.llm.toolcall",
				"idx", i,
				"name", tc.Name,
				"argKeys", strings.Join(keys, ","),
				"argSample", fmt.Sprintf("%+v", tc.Arguments),
			)
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
			logging.Info(ctx, "webchat.reasoning",
				"thread", job.ThreadID.String(),
				"turn", job.TurnID.String(),
				"round", rounds,
				"chars", utf8.RuneCountInString(text),
			)
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
				envelope, execErr := w.tools.Execute(tools.WithWorkspace(tools.WithThreadID(ctx, job.ThreadID), job.Workspace), service.ToolCall{
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
				logging.Info(ctx, "webchat.tool",
					"thread", job.ThreadID.String(),
					"turn", job.TurnID.String(),
					"round", rounds,
					"tool", tc.Name,
					"call_id", callID,
					"ok", envelope.OK,
					"args_bytes", len(mustJSON(tc.Arguments)),
					"result_bytes", len(raw),
				)
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
				logging.Info(ctx, "webchat.tool_dedupe",
					"thread", job.ThreadID.String(),
					"turn", job.TurnID.String(),
					"round", rounds,
					"fingerprint", fp,
				)
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

		text := strings.TrimSpace(resp.Text)
		if text == "" {
			hasReasoning := strings.TrimSpace(resp.Reasoning) != ""
			logging.Warn(ctx, "webchat.empty_model_response",
				"thread", job.ThreadID.String(),
				"turn", job.TurnID.String(),
				"round", rounds,
				"provider", resp.Model.Provider,
				"model", resp.Model.ID,
				"api", resp.Model.API,
				"has_reasoning", hasReasoning,
				"reasoning_chars", utf8.RuneCountInString(resp.Reasoning),
				"tool_calls_len", len(resp.ToolCalls),
				"finish_reason", resp.Status,
				"nudged", emptyNudged,
			)
			// Reasoning-only / truncated rounds: give the model one chance to answer or tool-call.
			if !emptyNudged && rounds < maxRounds {
				emptyNudged = true
				asst := map[string]any{"role": "assistant", "content": ""}
				if hasReasoning {
					asst["reasoning"] = resp.Reasoning
				}
				messages = append(messages, asst)
				messages = append(messages, map[string]any{
					"role": "system",
					"content": "You produced reasoning but no user-facing answer and no tool call. " +
						"Answer the user now in plain text, or call a tool with concrete arguments.",
				})
				logging.Info(ctx, "webchat.empty_response_nudge",
					"thread", job.ThreadID.String(),
					"turn", job.TurnID.String(),
					"round", rounds,
				)
				continue
			}
			text = "(empty model response)"
			if _, err := w.appendID(ctx, job, enum.ItemAgentMessage, "", map[string]any{
				"text":     text,
				"origin":   "runtime",
				"model":    modelMetadata(resp, "response"),
				"trace_id": logging.TraceID(ctx),
			}); err != nil {
				return err
			}
			lastToolOnly = false
			break
		}
		agentItemID := ""
		if streamedText {
			agentItemID = draftItemID
		}
		if _, err := w.appendID(ctx, job, enum.ItemAgentMessage, agentItemID, map[string]any{
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

func (w *Worker) append(ctx context.Context, job service.TurnJob, typ enum.ItemType, payload map[string]any) (entity.TranscriptItem, error) {
	return w.appendID(ctx, job, typ, "", payload)
}

func (w *Worker) appendID(ctx context.Context, job service.TurnJob, typ enum.ItemType, itemID string, payload map[string]any) (entity.TranscriptItem, error) {
	var id valueobject.ItemID
	var err error
	if strings.TrimSpace(itemID) != "" {
		id, err = valueobject.NewItemID(itemID)
	} else {
		id, err = valueobject.NewItemID(idgen.ItemID())
	}
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

func (w *Worker) buildMessages(ctx context.Context, threadID valueobject.ThreadID, items []entity.TranscriptItem) []map[string]any {
	var loader *visionLoader
	if w.attachments != nil {
		skip := false
		if w.vision != nil {
			skip = !w.vision.AllowPixels(ctx)
		} else if config.ParseVisionMode(w.cfg.LLMVision) == "off" {
			skip = true
		}
		loader = &visionLoader{
			ctx:        ctx,
			threadID:   threadID,
			store:      w.attachments,
			skipPixels: skip,
		}
	}
	return buildMessages(items, loader)
}

func buildMessages(items []entity.TranscriptItem, loader *visionLoader) []map[string]any {
	lastCompact := -1
	for i, it := range items {
		if it.Type == enum.ItemContextCompact {
			lastCompact = i
		}
	}
	if lastCompact < 0 {
		var msgs []map[string]any
		for _, it := range items {
			appendMessageFromItem(&msgs, it, loader)
		}
		return msgs
	}

	compact := items[lastCompact]
	text, _ := compact.Payload["text"].(string)
	throughSeq := asUint64(compact.Payload["compacted_through_seq"])

	msgs := []map[string]any{{
		"role":    "system",
		"content": "Conversation summary:\n" + text,
	}}
	for i, it := range items {
		if it.Type == enum.ItemContextCompact {
			continue
		}
		if throughSeq > 0 {
			if it.Seq > 0 && it.Seq <= throughSeq {
				continue
			}
			if it.Seq == 0 && i < lastCompact {
				continue
			}
		} else if i < lastCompact {
			// Legacy checkpoints without compacted_through_seq: keep same-turn
			// items before the compact event (AIPedia-compatible).
			if it.TurnID != compact.TurnID {
				continue
			}
		}
		appendMessageFromItem(&msgs, it, loader)
	}
	return msgs
}

func appendMessageFromItem(msgs *[]map[string]any, it entity.TranscriptItem, loader *visionLoader) {
	switch it.Type {
	case enum.ItemUserMessage:
		*msgs = append(*msgs, map[string]any{"role": "user", "content": userLLMContent(it.Payload, loader)})
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

func (w *Worker) attachmentRefs(ctx context.Context, job service.TurnJob) []map[string]any {
	if w.attachments == nil || len(job.AttachmentIDs) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(job.AttachmentIDs))
	for _, id := range job.AttachmentIDs {
		meta, _, err := w.attachments.ResolvePath(ctx, job.ThreadID, id)
		if err != nil {
			out = append(out, map[string]any{
				"id":            id,
				"attachment_id": id,
				"missing":       true,
			})
			continue
		}
		out = append(out, map[string]any{
			"id":            meta.ID,
			"attachment_id": meta.ID,
			"filename":      meta.Filename,
			"mime":          meta.Mime,
			"size":          meta.Size,
			"kind":          meta.Kind,
		})
	}
	return out
}

// userContentFromPayload builds the LLM user message. Plain text alone hides
// durable attachments from the model — prompts tell it to call read_attachment /
// read_image with attachment_id from an attachments list on the user_message.
func userContentFromPayload(payload map[string]any) string {
	text, _ := payload["text"].(string)
	refs := llmAttachmentRefs(payload["attachments"])
	if len(refs) == 0 {
		return text
	}
	raw, err := json.Marshal(refs)
	if err != nil {
		return text
	}
	block := "attachments: " + string(raw)
	text = strings.TrimSpace(text)
	if text == "" {
		return block
	}
	return text + "\n\n" + block
}

func llmAttachmentRefs(raw any) []map[string]any {
	list, ok := raw.([]any)
	if !ok {
		// jsonl / tests may already be []map[string]any
		if typed, ok := raw.([]map[string]any); ok {
			out := make([]map[string]any, 0, len(typed))
			for _, m := range typed {
				if ref := normalizeLLMAttachmentRef(m); ref != nil {
					out = append(out, ref)
				}
			}
			return out
		}
		return nil
	}
	out := make([]map[string]any, 0, len(list))
	for _, el := range list {
		m, ok := el.(map[string]any)
		if !ok {
			continue
		}
		if ref := normalizeLLMAttachmentRef(m); ref != nil {
			out = append(out, ref)
		}
	}
	return out
}

func normalizeLLMAttachmentRef(m map[string]any) map[string]any {
	id, _ := m["attachment_id"].(string)
	if id == "" {
		id, _ = m["id"].(string)
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	ref := map[string]any{"attachment_id": id}
	if v, ok := m["filename"]; ok {
		ref["filename"] = v
	}
	if v, ok := m["mime"]; ok {
		ref["mime"] = v
	}
	if v, ok := m["size"]; ok {
		ref["size"] = v
	}
	if v, ok := m["kind"]; ok {
		ref["kind"] = v
	}
	if missing, ok := m["missing"].(bool); ok && missing {
		ref["missing"] = true
	}
	return ref
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
