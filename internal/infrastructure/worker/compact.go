package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/enum"
	"buatpostingan/internal/domain/service"
	"buatpostingan/internal/pkg/logging"
)

// maybeCompact appends a durable context_compacted checkpoint when the
// transcript exceeds the configured input budget. Stub mode is a no-op.
// Failures fall back to extractive summary so the turn still proceeds.
func (w *Worker) maybeCompact(ctx context.Context, job service.TurnJob, items []entity.TranscriptItem) ([]entity.TranscriptItem, error) {
	if w == nil || !w.cfg.ContextCompactionEnabled || w.cfg.LLMStub {
		return items, nil
	}
	threshold := w.cfg.ContextMaxInputTokens - w.cfg.ContextReserveTokens
	if threshold < 1000 {
		threshold = 1000
	}
	est := estimateItemTokens(items)
	if est <= threshold {
		return items, nil
	}

	recent := w.cfg.ContextRecentTurns
	if recent < 1 {
		recent = 4
	}
	turnIDs := uniqueTurnIDs(items)
	if len(turnIDs) <= recent {
		logging.Info(ctx, "webchat.compact",
			"thread", job.ThreadID.String(),
			"turn", job.TurnID.String(),
			"skipped", "too_few_turns",
			"turns", len(turnIDs),
			"tokens", est,
		)
		return items, nil
	}
	keepFrom := len(turnIDs) - recent
	oldSet := make(map[string]struct{}, keepFrom)
	for _, id := range turnIDs[:keepFrom] {
		oldSet[id] = struct{}{}
	}

	var old []entity.TranscriptItem
	var throughSeq uint64
	for _, it := range items {
		if _, ok := oldSet[it.TurnID.String()]; !ok {
			continue
		}
		old = append(old, it)
		if it.Seq > throughSeq {
			throughSeq = it.Seq
		}
	}
	if len(old) == 0 {
		return items, nil
	}

	excerpt := buildCompactExcerpt(old, w.cfg.ContextSummaryMaxChars)
	if strings.TrimSpace(excerpt) == "" {
		return items, nil
	}

	summary, via := w.summarizeForCompact(ctx, job, excerpt)
	summary = clampRunes(summary, w.cfg.ContextSummaryMaxChars)
	if strings.TrimSpace(summary) == "" {
		summary = clampRunes(excerpt, w.cfg.ContextSummaryMaxChars)
		via = "extractive"
	}

	payload := map[string]any{
		"text":                   summary,
		"origin":                 "runtime",
		"via":                    via,
		"compacted_through_seq":  throughSeq,
		"estimated_input_tokens": est,
	}
	item, err := w.append(ctx, job, enum.ItemContextCompact, payload)
	if err != nil {
		logging.Error(ctx, "webchat.compact", err,
			"thread", job.ThreadID.String(),
			"turn", job.TurnID.String(),
		)
		// Persist failed — continue with uncompacted history rather than fail the turn.
		return items, nil
	}
	logging.Info(ctx, "webchat.compact",
		"thread", job.ThreadID.String(),
		"turn", job.TurnID.String(),
		"via", via,
		"tokens_before", est,
		"threshold", threshold,
		"through_seq", throughSeq,
		"summary_chars", utf8.RuneCountInString(summary),
	)
	return append(items, item), nil
}

func (w *Worker) summarizeForCompact(ctx context.Context, job service.TurnJob, excerpt string) (string, string) {
	if w.llm == nil {
		return excerpt, "extractive"
	}
	prompt := loadCompactPrompt(w.cfg.PromptsRoot)
	msgs := []map[string]any{
		{"role": "system", "content": prompt},
		{"role": "user", "content": excerpt},
	}
	resp, err := w.llm.Chat(ctx, msgs, nil, strings.TrimSpace(job.ProviderID))
	if err != nil {
		logging.Error(ctx, "webchat.compact", err,
			"thread", job.ThreadID.String(),
			"turn", job.TurnID.String(),
			"fallback", "extractive",
		)
		return excerpt, "extractive"
	}
	text := strings.TrimSpace(resp.Text)
	if text == "" {
		return excerpt, "extractive"
	}
	return text, "llm"
}

func loadCompactPrompt(promptsRoot string) string {
	const fallback = "You are performing a CONTEXT CHECKPOINT COMPACTION. Create a concise handoff summary for another LLM. Reply with the summary only."
	if strings.TrimSpace(promptsRoot) == "" {
		return fallback
	}
	raw, err := os.ReadFile(filepath.Join(promptsRoot, "compact.md"))
	if err != nil {
		return fallback
	}
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return fallback
	}
	return text
}

func uniqueTurnIDs(items []entity.TranscriptItem) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, it := range items {
		id := it.TurnID.String()
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func buildCompactExcerpt(items []entity.TranscriptItem, maxChars int) string {
	var parts []string
	for _, it := range items {
		switch it.Type {
		case enum.ItemContextCompact:
			text, _ := it.Payload["text"].(string)
			if strings.TrimSpace(text) != "" {
				parts = append(parts, strings.TrimSpace(text))
			}
		case enum.ItemUserMessage:
			text, _ := it.Payload["text"].(string)
			parts = append(parts, "User: "+strings.TrimSpace(text))
		case enum.ItemAgentMessage:
			text, _ := it.Payload["text"].(string)
			parts = append(parts, "Assistant: "+strings.TrimSpace(text))
		case enum.ItemToolCall:
			name, _ := it.Payload["name"].(string)
			if name == "" {
				name = "unknown"
			}
			parts = append(parts, "Tool call: "+name)
		case enum.ItemToolResult:
			raw, _ := json.Marshal(it.Payload["envelope"])
			parts = append(parts, "Tool result: "+clampRunes(string(raw), 400))
		case enum.ItemReasoning:
			text, _ := it.Payload["text"].(string)
			if strings.TrimSpace(text) != "" {
				parts = append(parts, "Reasoning: "+clampRunes(strings.TrimSpace(text), 400))
			}
		}
	}
	return clampRunes(strings.TrimSpace(strings.Join(parts, "\n")), maxChars)
}

func estimateItemTokens(items []entity.TranscriptItem) int {
	chars := 0
	for _, it := range items {
		if text, ok := it.Payload["text"].(string); ok {
			chars += len(text)
		}
		if args, ok := it.Payload["arguments"]; ok {
			raw, _ := json.Marshal(args)
			chars += len(raw)
		}
		if env, ok := it.Payload["envelope"]; ok {
			raw, _ := json.Marshal(env)
			chars += len(raw)
		}
	}
	return (chars + 3) / 4
}

func clampRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 {
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

func asUint64(v any) uint64 {
	switch n := v.(type) {
	case uint64:
		return n
	case int:
		if n > 0 {
			return uint64(n)
		}
	case int64:
		if n > 0 {
			return uint64(n)
		}
	case float64:
		if n > 0 {
			return uint64(n)
		}
	case json.Number:
		i, err := n.Int64()
		if err == nil && i > 0 {
			return uint64(i)
		}
	case string:
		var u uint64
		_, _ = fmt.Sscanf(n, "%d", &u)
		return u
	}
	return 0
}
