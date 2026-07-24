package worker

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"buatpostingan/internal/domain/enum"
	"buatpostingan/internal/domain/service"
	"buatpostingan/internal/domain/valueobject"
	"buatpostingan/internal/pkg/logging"
)

// maybeAutoTitle titles a conversation after the first completed turn.
// Stub path applies a sync truncate. Real LLM path schedules an async job
// that inherits the turn's trace_id and never blocks / fails the turn.
func (w *Worker) maybeAutoTitle(ctx context.Context, job service.TurnJob) {
	prev, ok, err := w.store.ResolveConversation(ctx, job.ThreadID)
	if err != nil || !ok {
		return
	}
	if prev.TitleSource == enum.TitleManual {
		return
	}
	if prev.TitleSource == enum.TitleAuto && prev.Title != nil && strings.TrimSpace(prev.Title.String()) != "" {
		return
	}
	if prev.TitleSource != enum.TitlePending && prev.TitleSource != "" {
		return
	}

	userText := strings.TrimSpace(job.Message)
	if userText == "" {
		userText, _ = w.findTurnUserText(ctx, job)
	}
	if strings.TrimSpace(userText) == "" {
		return
	}
	agentText := w.findFirstAgentText(ctx, job.ThreadID)

	if w.cfg.LLMStub || w.llm == nil {
		w.applyAutoTitle(ctx, job.ThreadID, truncateRunes(userText, 60), "truncate")
		return
	}

	traceID := logging.TraceID(ctx)
	if traceID == "" {
		traceID = job.TraceID
	}
	if traceID == "" {
		traceID = logging.TraceSystem
	}
	go w.runAutoTitleJob(traceID, job, userText, agentText)
}

func (w *Worker) runAutoTitleJob(traceID string, job service.TurnJob, userText, agentText string) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	ctx = logging.WithTraceID(ctx, traceID)

	defer func() {
		if rec := recover(); rec != nil {
			logging.Error(ctx, "webchat.title", fmt.Errorf("%v", rec),
				"thread", job.ThreadID.String(),
			)
			_ = w.applyAutoTitle(ctx, job.ThreadID, truncateRunes(userText, 60), "truncate")
		}
	}()

	title, via := w.generateTitle(ctx, job, userText, agentText)
	if err := w.applyAutoTitle(ctx, job.ThreadID, title, via); err != nil {
		logging.Error(ctx, "webchat.title", err,
			"thread", job.ThreadID.String(),
		)
		return
	}
	logging.Info(ctx, "webchat.title_applied",
		"thread", job.ThreadID.String(),
		"via", via,
		"title_chars", utf8.RuneCountInString(title),
	)
	if w.hub != nil {
		w.hub.PublishEphemeral(job.ThreadID, "conversation.updated", map[string]any{
			"thread_id":    job.ThreadID.String(),
			"title":        title,
			"title_source": string(enum.TitleAuto),
		})
	}
}

func (w *Worker) generateTitle(ctx context.Context, job service.TurnJob, userText, agentText string) (string, string) {
	userClip := clampRunes(userText, 400)
	agentClip := clampRunes(agentText, 200)
	msgs := []map[string]any{
		{
			"role":    "system",
			"content": "Title this chat in at most 6 words. No quotes, no punctuation fluff, no tools. Reply with the title only.",
		},
		{
			"role":    "user",
			"content": "User: " + userClip + "\nAssistant: " + agentClip + "\nTitle:",
		},
	}
	resp, err := w.llm.Chat(ctx, msgs, nil, strings.TrimSpace(job.ProviderID))
	if err != nil {
		logging.Error(ctx, "webchat.title", err,
			"thread", job.ThreadID.String(),
			"fallback", "truncate",
		)
		return truncateRunes(userText, 60), "truncate"
	}
	raw := strings.TrimSpace(resp.Text)
	raw = strings.ReplaceAll(raw, "\r", " ")
	raw = strings.ReplaceAll(raw, "\n", " ")
	raw = strings.Join(strings.Fields(raw), " ")
	raw = strings.Trim(raw, `"'`+"“”")
	raw = clampRunes(raw, 60)
	if raw == "" {
		return truncateRunes(userText, 60), "truncate"
	}
	if _, err := valueobject.NewTitle(raw); err != nil {
		return truncateRunes(userText, 60), "truncate"
	}
	return raw, "llm"
}

func (w *Worker) applyAutoTitle(ctx context.Context, threadID valueobject.ThreadID, titleRaw, via string) error {
	prev, ok, err := w.store.ResolveConversation(ctx, threadID)
	if err != nil {
		return err
	}
	if !ok {
		return errf("conversation not found")
	}
	if prev.TitleSource == enum.TitleManual {
		logging.Info(ctx, "webchat.title_skip",
			"thread", threadID.String(),
			"reason", "manual_locked",
			"via", via,
		)
		return nil
	}
	if prev.Status == enum.ConversationDeleted {
		logging.Info(ctx, "webchat.title_skip",
			"thread", threadID.String(),
			"reason", "deleted",
		)
		return nil
	}
	title, err := valueobject.NewTitle(titleRaw)
	if err != nil {
		title, err = valueobject.NewTitle(truncateRunes(titleRaw, 60))
		if err != nil {
			return err
		}
	}
	prev.Title = &title
	prev.TitleSource = enum.TitleAuto
	now := time.Now().UTC()
	prev.UpdatedAt = now
	prev.LastActivityAt = now
	return w.store.AppendConversationMeta(ctx, prev)
}

func (w *Worker) findFirstAgentText(ctx context.Context, threadID valueobject.ThreadID) string {
	snap, err := w.store.GetThread(ctx, threadID, 0)
	if err != nil {
		return ""
	}
	for _, it := range snap.Items {
		if it.Type != enum.ItemAgentMessage {
			continue
		}
		text, _ := it.Payload["text"].(string)
		if strings.TrimSpace(text) != "" {
			return text
		}
	}
	return ""
}
