package worker

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"buatpostingan/internal/config"
	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/enum"
	"buatpostingan/internal/domain/service"
	"buatpostingan/internal/domain/valueobject"
	"buatpostingan/internal/pkg/logging"
)

func TestEstimateAndExcerpt(t *testing.T) {
	tid, _ := valueobject.NewThreadID("thr_est")
	turn, _ := valueobject.NewTurnID("trn_est")
	items := []entity.TranscriptItem{
		{Type: enum.ItemUserMessage, ThreadID: tid, TurnID: turn, Payload: map[string]any{"text": strings.Repeat("a", 40)}},
		{Type: enum.ItemToolCall, ThreadID: tid, TurnID: turn, Payload: map[string]any{
			"name": "search_docs", "arguments": map[string]any{"q": "x"},
		}},
		{Type: enum.ItemToolResult, ThreadID: tid, TurnID: turn, Payload: map[string]any{
			"envelope": map[string]any{"ok": true},
		}},
	}
	if estimateItemTokens(items) < 10 {
		t.Fatalf("tokens too low")
	}
	ex := buildCompactExcerpt(items, 100)
	if !strings.Contains(ex, "User:") || !strings.Contains(ex, "Tool call:") {
		t.Fatalf("excerpt=%q", ex)
	}
	if asUint64(float64(12)) != 12 || asUint64("9") != 9 || asUint64(nil) != 0 {
		t.Fatal("asUint64")
	}
}

func TestBuildMessagesKeepsRecentViaThroughSeq(t *testing.T) {
	tid, _ := valueobject.NewThreadID("thr_c")
	oldTurn, _ := valueobject.NewTurnID("trn_old")
	recentTurn, _ := valueobject.NewTurnID("trn_recent")
	curTurn, _ := valueobject.NewTurnID("trn_cur")
	items := []entity.TranscriptItem{
		{Seq: 1, Type: enum.ItemUserMessage, ThreadID: tid, TurnID: oldTurn, Payload: map[string]any{"text": "old user"}},
		{Seq: 2, Type: enum.ItemAgentMessage, ThreadID: tid, TurnID: oldTurn, Payload: map[string]any{"text": "old agent"}},
		{Seq: 3, Type: enum.ItemUserMessage, ThreadID: tid, TurnID: recentTurn, Payload: map[string]any{"text": "recent user"}},
		{Seq: 4, Type: enum.ItemAgentMessage, ThreadID: tid, TurnID: recentTurn, Payload: map[string]any{"text": "recent agent"}},
		{Seq: 5, Type: enum.ItemUserMessage, ThreadID: tid, TurnID: curTurn, Payload: map[string]any{"text": "latest"}},
		{Seq: 6, Type: enum.ItemContextCompact, ThreadID: tid, TurnID: curTurn, Payload: map[string]any{
			"text": "SUM", "compacted_through_seq": uint64(2),
		}},
	}
	msgs := buildMessages(items, nil)
	if len(msgs) != 4 { // summary + recent user/agent + latest
		t.Fatalf("len=%d %#v", len(msgs), msgs)
	}
	if msgs[0]["role"] != "system" || !strings.Contains(msgs[0]["content"].(string), "SUM") {
		t.Fatalf("%#v", msgs[0])
	}
	joined := ""
	for _, m := range msgs {
		if s, ok := m["content"].(string); ok {
			joined += s + "\n"
		}
	}
	if strings.Contains(joined, "old user") {
		t.Fatalf("old turn should be compacted away: %q", joined)
	}
	if !strings.Contains(joined, "recent user") || !strings.Contains(joined, "latest") {
		t.Fatalf("recent/latest missing: %q", joined)
	}
}

func TestMaybeCompactLLMWritesCheckpoint(t *testing.T) {
	job := sampleJob()
	old1, _ := valueobject.NewTurnID("trn_o1")
	old2, _ := valueobject.NewTurnID("trn_o2")
	old3, _ := valueobject.NewTurnID("trn_o3")
	old4, _ := valueobject.NewTurnID("trn_o4")
	blob := strings.Repeat("word ", 800)
	items := []entity.TranscriptItem{
		{Seq: 1, Type: enum.ItemUserMessage, ThreadID: job.ThreadID, TurnID: old1, Payload: map[string]any{"text": blob}},
		{Seq: 2, Type: enum.ItemAgentMessage, ThreadID: job.ThreadID, TurnID: old1, Payload: map[string]any{"text": blob}},
		{Seq: 3, Type: enum.ItemUserMessage, ThreadID: job.ThreadID, TurnID: old2, Payload: map[string]any{"text": blob}},
		{Seq: 4, Type: enum.ItemAgentMessage, ThreadID: job.ThreadID, TurnID: old2, Payload: map[string]any{"text": blob}},
		{Seq: 5, Type: enum.ItemUserMessage, ThreadID: job.ThreadID, TurnID: old3, Payload: map[string]any{"text": blob}},
		{Seq: 6, Type: enum.ItemAgentMessage, ThreadID: job.ThreadID, TurnID: old3, Payload: map[string]any{"text": blob}},
		{Seq: 7, Type: enum.ItemUserMessage, ThreadID: job.ThreadID, TurnID: old4, Payload: map[string]any{"text": "keep me"}},
		{Seq: 8, Type: enum.ItemAgentMessage, ThreadID: job.ThreadID, TurnID: old4, Payload: map[string]any{"text": "recent"}},
		{Seq: 9, Type: enum.ItemUserMessage, ThreadID: job.ThreadID, TurnID: job.TurnID, Payload: map[string]any{"text": job.Message}},
	}
	store := &fakeStore{items: append([]entity.TranscriptItem{}, items...)}
	llm := &scriptLLM{resps: []service.LLMResult{{Text: "HANDOFF SUMMARY"}}}
	w := New(Deps{
		Config: config.Config{
			ContextCompactionEnabled: true,
			ContextMaxInputTokens:    500,
			ContextReserveTokens:     100,
			ContextRecentTurns:       2,
			ContextSummaryMaxChars:   2000,
			LLMStub:                  false,
		},
		Store: store,
		LLM:   llm,
	})
	out, err := w.maybeCompact(context.Background(), job, items)
	if err != nil {
		t.Fatal(err)
	}
	if llm.calls != 1 {
		t.Fatalf("llm calls=%d", llm.calls)
	}
	found := false
	for _, it := range out {
		if it.Type == enum.ItemContextCompact {
			found = true
			if it.Payload["text"] != "HANDOFF SUMMARY" {
				t.Fatalf("%#v", it.Payload)
			}
			if asUint64(it.Payload["compacted_through_seq"]) == 0 {
				t.Fatalf("missing through_seq %#v", it.Payload)
			}
		}
	}
	if !found {
		t.Fatal("expected context_compacted item")
	}
	store.mu.Lock()
	persisted := false
	for _, it := range store.items {
		if it.Type == enum.ItemContextCompact {
			persisted = true
		}
	}
	store.mu.Unlock()
	if !persisted {
		t.Fatal("compact not persisted")
	}
}

func TestMaybeCompactSkippedWhenStubOrDisabled(t *testing.T) {
	job := sampleJob()
	items := []entity.TranscriptItem{
		{Seq: 1, Type: enum.ItemUserMessage, ThreadID: job.ThreadID, TurnID: job.TurnID, Payload: map[string]any{"text": strings.Repeat("x", 5000)}},
	}
	store := &fakeStore{items: items}
	llm := &scriptLLM{resps: []service.LLMResult{{Text: "should not run"}}}
	w := New(Deps{
		Config: config.Config{ContextCompactionEnabled: true, LLMStub: true, ContextMaxInputTokens: 10, ContextReserveTokens: 0, ContextRecentTurns: 1},
		Store:  store, LLM: llm,
	})
	out, err := w.maybeCompact(context.Background(), job, items)
	if err != nil {
		t.Fatal(err)
	}
	if llm.calls != 0 || len(out) != len(items) {
		t.Fatalf("stub should no-op calls=%d len=%d", llm.calls, len(out))
	}
	w.cfg.LLMStub = false
	w.cfg.ContextCompactionEnabled = false
	out, err = w.maybeCompact(context.Background(), job, items)
	if err != nil || llm.calls != 0 || len(out) != len(items) {
		t.Fatalf("disabled should no-op")
	}
}

func TestMaybeCompactExtractiveFallback(t *testing.T) {
	job := sampleJob()
	var items []entity.TranscriptItem
	seq := uint64(0)
	blob := strings.Repeat("z", 2000)
	for i := 0; i < 5; i++ {
		turn, _ := valueobject.NewTurnID(fmt.Sprintf("trn_fb%d", i))
		seq++
		items = append(items, entity.TranscriptItem{
			Seq: seq, Type: enum.ItemUserMessage, ThreadID: job.ThreadID, TurnID: turn,
			Payload: map[string]any{"text": blob},
		})
		seq++
		items = append(items, entity.TranscriptItem{
			Seq: seq, Type: enum.ItemAgentMessage, ThreadID: job.ThreadID, TurnID: turn,
			Payload: map[string]any{"text": "ans"},
		})
	}
	store := &fakeStore{items: append([]entity.TranscriptItem{}, items...)}
	llm := &scriptLLM{errs: []error{errf("llm down")}}
	w := New(Deps{
		Config: config.Config{
			ContextCompactionEnabled: true,
			ContextMaxInputTokens:    200,
			ContextReserveTokens:     50,
			ContextRecentTurns:       2,
			ContextSummaryMaxChars:   500,
		},
		Store: store, LLM: llm,
	})
	out, err := w.maybeCompact(logging.WithTraceID(context.Background(), "tr_compact"), job, items)
	if err != nil {
		t.Fatal(err)
	}
	var compact entity.TranscriptItem
	for _, it := range out {
		if it.Type == enum.ItemContextCompact {
			compact = it
		}
	}
	if compact.Type != enum.ItemContextCompact {
		t.Fatal("missing compact")
	}
	if compact.Payload["via"] != "extractive" {
		t.Fatalf("via=%v", compact.Payload["via"])
	}
	text, _ := compact.Payload["text"].(string)
	if !strings.Contains(text, "User:") {
		t.Fatalf("extractive text=%q", text)
	}
}

func TestLoadCompactPromptFallback(t *testing.T) {
	p := loadCompactPrompt(filepath.Join(t.TempDir(), "missing"))
	if !strings.Contains(p, "CONTEXT CHECKPOINT") {
		t.Fatalf("%q", p)
	}
}

type titleHub struct {
	mu     sync.Mutex
	events []string
}

func (h *titleHub) Subscribe(valueobject.ThreadID) service.ThreadEventSub { return nil }
func (h *titleHub) Notify(valueobject.ThreadID)                           {}
func (h *titleHub) PublishEphemeral(_ valueobject.ThreadID, name string, _ map[string]any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.events = append(h.events, name)
}

func TestMaybeAutoTitleAsyncLLM(t *testing.T) {
	job := sampleJob()
	job.Message = "How do I configure SSO?"
	store := &fakeStore{
		meta: map[string]entity.ConversationMeta{
			job.ThreadID.String(): {ThreadID: job.ThreadID, TitleSource: enum.TitlePending, Status: enum.ConversationActive},
		},
		items: []entity.TranscriptItem{
			{Seq: 1, Type: enum.ItemUserMessage, ThreadID: job.ThreadID, TurnID: job.TurnID, Payload: map[string]any{"text": job.Message}},
			{Seq: 2, Type: enum.ItemAgentMessage, ThreadID: job.ThreadID, TurnID: job.TurnID, Payload: map[string]any{"text": "Use the settings panel."}},
		},
	}
	llm := &scriptLLM{resps: []service.LLMResult{{Text: "SSO Configuration Guide"}}}
	hub := &titleHub{}
	w := New(Deps{
		Config: config.Config{LLMStub: false},
		Store:  store,
		LLM:    llm,
		Hub:    hub,
	})
	ctx := logging.WithTraceID(context.Background(), "tr_title_parent")
	w.maybeAutoTitle(ctx, job)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		store.mu.Lock()
		meta := store.meta[job.ThreadID.String()]
		store.mu.Unlock()
		if meta.Title != nil && meta.TitleSource == enum.TitleAuto {
			if meta.Title.String() != "SSO Configuration Guide" {
				t.Fatalf("title=%q", meta.Title.String())
			}
			hub.mu.Lock()
			n := len(hub.events)
			hub.mu.Unlock()
			if n < 1 {
				t.Fatal("expected conversation.updated ephemeral")
			}
			if llm.calls != 1 {
				t.Fatalf("calls=%d", llm.calls)
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("async title did not apply")
}

func TestMaybeAutoTitleStubTruncate(t *testing.T) {
	job := sampleJob()
	job.Message = "hello title world"
	store := &fakeStore{
		meta: map[string]entity.ConversationMeta{
			job.ThreadID.String(): {ThreadID: job.ThreadID, TitleSource: enum.TitlePending},
		},
	}
	w := New(Deps{Config: config.Config{LLMStub: true}, Store: store})
	w.maybeAutoTitle(context.Background(), job)
	meta := store.meta[job.ThreadID.String()]
	if meta.Title == nil || meta.Title.String() != "hello title world" || meta.TitleSource != enum.TitleAuto {
		t.Fatalf("%#v", meta)
	}
}
