package worker

import (
	"context"
	"errors"
	"os"
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

type fakeStore struct {
	mu          sync.Mutex
	items       []entity.TranscriptItem
	meta        map[string]entity.ConversationMeta
	appendFn    func(entity.TranscriptItem) error
	getErr      error
	metaErr     error
	cleared     int
	released    int
	lastTraceID string
}

func (f *fakeStore) CreateThread(context.Context, int64) (entity.ThreadSnapshot, error) {
	return entity.ThreadSnapshot{}, errors.New("unused")
}
func (f *fakeStore) GetThread(_ context.Context, tid valueobject.ThreadID, afterSeq uint64) (entity.ThreadSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return entity.ThreadSnapshot{}, f.getErr
	}
	var out []entity.TranscriptItem
	for _, it := range f.items {
		if it.ThreadID == tid && it.Seq > afterSeq {
			out = append(out, it)
		}
	}
	return entity.ThreadSnapshot{ThreadID: tid, Items: out}, nil
}
func (f *fakeStore) AppendItem(ctx context.Context, tid valueobject.ThreadID, item entity.TranscriptItem) (entity.TranscriptItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if id := logging.TraceID(ctx); id != "" {
		f.lastTraceID = id
	}
	if f.appendFn != nil {
		if err := f.appendFn(item); err != nil {
			return entity.TranscriptItem{}, err
		}
	}
	item.Seq = uint64(len(f.items) + 1)
	item.ThreadID = tid
	f.items = append(f.items, item)
	return item, nil
}
func (f *fakeStore) ListConversations(context.Context) ([]entity.ConversationMeta, error) {
	return nil, nil
}
func (f *fakeStore) RenameThread(context.Context, valueobject.ThreadID, valueobject.Title) error {
	return nil
}
func (f *fakeStore) SoftDeleteThread(context.Context, valueobject.ThreadID) error { return nil }
func (f *fakeStore) SeqHead(context.Context, valueobject.ThreadID) (uint64, error) {
	return 0, nil
}
func (f *fakeStore) ResolveConversation(_ context.Context, tid valueobject.ThreadID) (entity.ConversationMeta, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.metaErr != nil {
		return entity.ConversationMeta{}, false, f.metaErr
	}
	if f.meta == nil {
		return entity.ConversationMeta{}, false, nil
	}
	m, ok := f.meta[tid.String()]
	return m, ok, nil
}
func (f *fakeStore) AppendConversationMeta(_ context.Context, meta entity.ConversationMeta) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.meta == nil {
		f.meta = map[string]entity.ConversationMeta{}
	}
	f.meta[meta.ThreadID.String()] = meta
	return nil
}
func (f *fakeStore) ClearActiveTurn(context.Context, valueobject.ThreadID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cleared++
	return nil
}

type fakeLock struct {
	released []string
}

func (l *fakeLock) TryAcquire(context.Context, valueobject.ThreadID) (string, error) {
	return "tok", nil
}
func (l *fakeLock) Release(_ context.Context, _ valueobject.ThreadID, tok string) error {
	l.released = append(l.released, tok)
	return nil
}
func (l *fakeLock) IsBusy(context.Context, valueobject.ThreadID) (bool, error) { return false, nil }

type fakeInterrupt struct {
	requested bool
	cleared   int
}

func (i *fakeInterrupt) Request(context.Context, valueobject.ThreadID, valueobject.TurnID) error {
	return nil
}
func (i *fakeInterrupt) IsRequested(context.Context, valueobject.ThreadID, valueobject.TurnID) (bool, error) {
	return i.requested, nil
}
func (i *fakeInterrupt) Clear(context.Context, valueobject.ThreadID, valueobject.TurnID) error {
	i.cleared++
	i.requested = false
	return nil
}

type toggleInterrupt struct {
	mu        sync.Mutex
	requested bool
	cleared   int
}

func (i *toggleInterrupt) Request(context.Context, valueobject.ThreadID, valueobject.TurnID) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.requested = true
	return nil
}
func (i *toggleInterrupt) IsRequested(context.Context, valueobject.ThreadID, valueobject.TurnID) (bool, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.requested, nil
}
func (i *toggleInterrupt) Clear(context.Context, valueobject.ThreadID, valueobject.TurnID) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.cleared++
	i.requested = false
	return nil
}
func (i *toggleInterrupt) SetRequested(v bool) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.requested = v
}

type fakeTools struct {
	schemas   []map[string]any
	schemaErr error
	exec      func(service.ToolCall) (service.ToolEnvelope, error)
	calls     []service.ToolCall
}

func (t *fakeTools) Schemas(context.Context) ([]map[string]any, error) {
	if t.schemaErr != nil {
		return nil, t.schemaErr
	}
	if t.schemas != nil {
		return t.schemas, nil
	}
	return []map[string]any{
		{"type": "function", "function": map[string]any{"name": "docs_search"}},
	}, nil
}
func (t *fakeTools) Execute(_ context.Context, call service.ToolCall) (service.ToolEnvelope, error) {
	t.calls = append(t.calls, call)
	if t.exec != nil {
		return t.exec(call)
	}
	return service.ToolEnvelope{OK: true, Tool: call.Name, Data: map[string]any{"ok": true}}, nil
}

type fakeDocs struct {
	count int
	err   error
}

func (d *fakeDocs) Gate(context.Context) (entity.DocsIndexGate, error) {
	if d.err != nil {
		return entity.DocsIndexGate{}, d.err
	}
	return entity.DocsIndexGate{Usable: true, DocumentCount: d.count}, nil
}
func (d *fakeDocs) Search(context.Context, string, int) (any, error) { return nil, nil }
func (d *fakeDocs) Reindex(context.Context) error                    { return nil }

type scriptLLM struct {
	mu    sync.Mutex
	resps []service.LLMResult
	errs  []error
	calls int
	seen  []string
}

func (l *scriptLLM) Chat(_ context.Context, _ []map[string]any, _ []map[string]any, pinned string) (service.LLMResult, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.seen = append(l.seen, pinned)
	i := l.calls
	l.calls++
	if i < len(l.errs) && l.errs[i] != nil {
		return service.LLMResult{}, l.errs[i]
	}
	if i < len(l.resps) {
		return l.resps[i], nil
	}
	return service.LLMResult{Text: "fallback"}, nil
}

// blockingLLM waits until its context is cancelled, then returns the error.
type blockingLLM struct {
	blockFor time.Duration
}

func (l *blockingLLM) Chat(ctx context.Context, _ []map[string]any, _ []map[string]any, _ string) (service.LLMResult, error) {
	timer := time.NewTimer(l.blockFor)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return service.LLMResult{}, ctx.Err()
	case <-timer.C:
		return service.LLMResult{Text: "never"}, nil
	}
}

func sampleJob() service.TurnJob {
	tid, _ := valueobject.NewThreadID("thr_test1")
	turn, _ := valueobject.NewTurnID("trn_test1")
	return service.TurnJob{
		ThreadID: tid, TurnID: turn, AdminUserID: 9, AdminName: "Ada",
		Message: "hello world", LockToken: "lock-1",
	}
}

func waitDone(t *testing.T, store *fakeStore, lock *fakeLock) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		store.mu.Lock()
		cleared := store.cleared
		store.mu.Unlock()
		if cleared > 0 && len(lock.released) > 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("worker did not finish")
}

func typesOf(store *fakeStore) []enum.ItemType {
	store.mu.Lock()
	defer store.mu.Unlock()
	out := make([]enum.ItemType, len(store.items))
	for i, it := range store.items {
		out[i] = it.Type
	}
	return out
}

func TestHelpers(t *testing.T) {
	if displayName("  ") != "Admin" || displayName(" Ada ") != "Ada" {
		t.Fatal("displayName")
	}
	if truncateRunes("", 10) != "New chat" {
		t.Fatal("empty truncate")
	}
	if truncateRunes("abcdefghij", 4) != "abcd" {
		t.Fatal("truncate")
	}
	u := emptyUsage()
	if u["input_tokens"] != 0 {
		t.Fatalf("%#v", u)
	}
	um := usageMap(service.TokenUsage{InputTokens: 1, OutputTokens: 2, ReasoningOutputTokens: 3})
	if um["input_tokens"] != 1 || um["reasoning_output_tokens"] != 3 {
		t.Fatalf("%#v", um)
	}
	md := modelMetadata(service.LLMResult{}, "response")
	if md["provider"] != "unknown" || md["id"] != "unknown" || md["api"] != "unknown" {
		t.Fatalf("%#v", md)
	}
	md = modelMetadata(service.LLMResult{
		ProviderID: "P",
		Model:      service.ModelRef{Provider: "Prov", ID: "m", API: "responses"},
	}, "planner")
	if md["provider"] != "Prov" || md["id"] != "m" || md["role"] != "planner" {
		t.Fatalf("%#v", md)
	}
	if string(mustJSON(map[string]any{"a": 1})) != `{"a":1}` {
		t.Fatal("mustJSON")
	}
	if string(mustJSON(func() {})) != "{}" {
		t.Fatal("mustJSON bad")
	}
	env := envelopeToMap(service.ToolEnvelope{
		OK: false, Tool: "t", Data: 1,
		Error: map[string]any{"code": "x"}, Meta: map[string]any{"m": 1},
	})
	if env["ok"] != false || env["error"] == nil || env["meta"] == nil {
		t.Fatalf("%#v", env)
	}
	fp := toolCallsFingerprint([]service.ToolCall{
		{Name: "a", Arguments: map[string]any{"q": 1}},
		{Name: "b", Arguments: map[string]any{"q": 2}},
	})
	if !strings.Contains(fp, "a") || !strings.Contains(fp, "b") {
		t.Fatalf("fp=%q", fp)
	}
	msg := assistantToolMessage([]service.ToolCall{{Name: "docs_search", Arguments: map[string]any{"q": "x"}}})
	tcs, _ := msg["tool_calls"].([]any)
	if len(tcs) != 1 {
		t.Fatalf("%#v", msg)
	}
	if errf("boom").Error() != "boom" {
		t.Fatal("errf")
	}
}

func TestBuildMessages(t *testing.T) {
	tid, _ := valueobject.NewThreadID("thr_1")
	turn, _ := valueobject.NewTurnID("trn_1")
	items := []entity.TranscriptItem{
		{Type: enum.ItemUserMessage, ThreadID: tid, TurnID: turn, Payload: map[string]any{"text": "hi"}},
		{Type: enum.ItemAgentMessage, ThreadID: tid, TurnID: turn, Payload: map[string]any{"text": "yo"}},
		{Type: enum.ItemToolCall, ThreadID: tid, TurnID: turn, Payload: map[string]any{
			"call_id": "c1", "name": "docs_search", "arguments": map[string]any{"q": "a"},
		}},
		{Type: enum.ItemToolResult, ThreadID: tid, TurnID: turn, Payload: map[string]any{
			"call_id": "c1", "envelope": map[string]any{"ok": true},
		}},
		{Type: enum.ItemReasoning, ThreadID: tid, TurnID: turn, Payload: map[string]any{"text": "think"}},
	}
	msgs := buildMessages(items, nil)
	if len(msgs) != 4 {
		t.Fatalf("len=%d %#v", len(msgs), msgs)
	}
	if msgs[0]["role"] != "user" || msgs[1]["role"] != "assistant" {
		t.Fatalf("%#v", msgs)
	}
	if msgs[2]["role"] != "assistant" {
		t.Fatalf("tool call msg %#v", msgs[2])
	}
	if msgs[3]["role"] != "tool" {
		t.Fatalf("%#v", msgs[3])
	}

	withAtt := []entity.TranscriptItem{
		{Type: enum.ItemUserMessage, ThreadID: tid, TurnID: turn, Payload: map[string]any{
			"text": "baca lampiran",
			"attachments": []any{
				map[string]any{
					"id":       "att_1",
					"filename": "note.md",
					"mime":     "text/markdown",
					"kind":     "text",
					"size":     12,
				},
			},
		}},
	}
	attMsgs := buildMessages(withAtt, nil)
	if len(attMsgs) != 1 {
		t.Fatalf("att len=%d", len(attMsgs))
	}
	content, _ := attMsgs[0]["content"].(string)
	if !strings.Contains(content, "baca lampiran") {
		t.Fatalf("missing text: %q", content)
	}
	if !strings.Contains(content, `"attachment_id":"att_1"`) {
		t.Fatalf("missing attachment_id for LLM: %q", content)
	}
	if !strings.Contains(content, "attachments:") {
		t.Fatalf("missing attachments block: %q", content)
	}

	compactTurn, _ := valueobject.NewTurnID("trn_2")
	items = append(items,
		entity.TranscriptItem{Type: enum.ItemUserMessage, ThreadID: tid, TurnID: compactTurn, Payload: map[string]any{"text": "old"}},
		entity.TranscriptItem{Type: enum.ItemContextCompact, ThreadID: tid, TurnID: compactTurn, Payload: map[string]any{"text": "summary"}},
		entity.TranscriptItem{Type: enum.ItemUserMessage, ThreadID: tid, TurnID: compactTurn, Payload: map[string]any{"text": "after"}},
	)
	msgs = buildMessages(items, nil)
	if msgs[0]["role"] != "system" || !strings.Contains(msgs[0]["content"].(string), "summary") {
		t.Fatalf("compact %#v", msgs)
	}
}

func TestNewAndEnqueueStub(t *testing.T) {
	store := &fakeStore{}
	lock := &fakeLock{}
	w := New(Deps{
		Config: config.Config{LLMStub: true, TurnJobTimeoutSec: 1},
		Store:  store, Locks: lock, Interrupt: &fakeInterrupt{},
	})
	job := sampleJob()
	if err := w.Enqueue(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	waitDone(t, store, lock)
	got := typesOf(store)
	want := []enum.ItemType{enum.ItemUserMessage, enum.ItemTurnStarted, enum.ItemAgentMessage, enum.ItemTurnCompleted}
	if len(got) != len(want) {
		t.Fatalf("got=%v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got=%v want=%v", got, want)
		}
	}
	store.mu.Lock()
	meta := store.meta[job.ThreadID.String()]
	trace := store.lastTraceID
	store.mu.Unlock()
	if meta.Title == nil || meta.TitleSource != enum.TitleAuto {
		t.Fatalf("auto title %#v", meta)
	}
	if lock.released[0] != "lock-1" {
		t.Fatalf("released=%v", lock.released)
	}
	if trace != logging.TraceSystem {
		t.Fatalf("background enqueue without ctx trace want system got %q", trace)
	}
}

func TestEnqueuePropagatesHTTPTraceID(t *testing.T) {
	store := &fakeStore{}
	lock := &fakeLock{}
	w := New(Deps{
		Config: config.Config{LLMStub: true, TurnJobTimeoutSec: 1},
		Store:  store, Locks: lock, Interrupt: &fakeInterrupt{},
	})
	ctx := logging.WithTraceID(context.Background(), "tr_http_request")
	if err := w.Enqueue(ctx, sampleJob()); err != nil {
		t.Fatal(err)
	}
	waitDone(t, store, lock)
	store.mu.Lock()
	got := store.lastTraceID
	store.mu.Unlock()
	if got != "tr_http_request" {
		t.Fatalf("trace=%q", got)
	}
}

func TestEnqueueUsesJobTraceID(t *testing.T) {
	store := &fakeStore{}
	lock := &fakeLock{}
	w := New(Deps{
		Config: config.Config{LLMStub: true, TurnJobTimeoutSec: 1},
		Store:  store, Locks: lock, Interrupt: &fakeInterrupt{},
	})
	job := sampleJob()
	job.TraceID = "tr_on_job"
	if err := w.Enqueue(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	waitDone(t, store, lock)
	store.mu.Lock()
	got := store.lastTraceID
	store.mu.Unlock()
	if got != "tr_on_job" {
		t.Fatalf("trace=%q", got)
	}
}

func TestProcessRetryAndFailure(t *testing.T) {
	job := sampleJob()
	store := &fakeStore{items: []entity.TranscriptItem{
		{Seq: 1, ThreadID: job.ThreadID, TurnID: job.TurnID, Type: enum.ItemUserMessage, Payload: map[string]any{"text": "prior"}},
	}}
	lock := &fakeLock{}
	w := New(Deps{
		Config: config.Config{LLMStub: true},
		Store:  store, Locks: lock, Interrupt: &fakeInterrupt{},
	})
	job.IsRetry = true
	job.Message = ""
	w.process(context.Background(), job)
	got := typesOf(store)
	if got[1] != enum.ItemTurnResumed {
		t.Fatalf("got=%v", got)
	}

	store2 := &fakeStore{}
	w2 := New(Deps{
		Config: config.Config{LLMStub: true},
		Store:  store2, Locks: &fakeLock{}, Interrupt: &fakeInterrupt{},
	})
	job2 := sampleJob()
	job2.IsRetry = true
	w2.process(context.Background(), job2)
	if typesOf(store2)[0] != enum.ItemTurnFailed {
		t.Fatalf("want turn.failed for missing user msg, got %v", typesOf(store2))
	}
}

func TestRunAgentTextPath(t *testing.T) {
	root := t.TempDir()
	mustWritePrompt(t, root)
	store := &fakeStore{}
	lock := &fakeLock{}
	llm := &scriptLLM{resps: []service.LLMResult{{
		Text: "answer", Reasoning: "think", ProviderID: "OR",
		Model: service.ModelRef{Provider: "OR", ID: "m", API: "responses"},
		Usage: service.TokenUsage{InputTokens: 2, OutputTokens: 3},
	}}}
	w := New(Deps{
		Config: config.Config{LLMStub: false, PromptsRoot: root, MaxToolRounds: 4},
		Store:  store, Locks: lock, Interrupt: &fakeInterrupt{},
		Tools: &fakeTools{}, Docs: &fakeDocs{count: 2}, LLM: llm,
	})
	w.process(context.Background(), sampleJob())
	got := typesOf(store)
	if !containsType(got, enum.ItemReasoning) || !containsType(got, enum.ItemAgentMessage) || !containsType(got, enum.ItemTurnCompleted) {
		t.Fatalf("got=%v", got)
	}
}

func TestRunAgentToolThenAnswer(t *testing.T) {
	root := t.TempDir()
	mustWritePrompt(t, root)
	store := &fakeStore{}
	llm := &scriptLLM{resps: []service.LLMResult{
		{ToolCalls: []service.ToolCall{{Name: "docs_search", Arguments: map[string]any{"q": "x"}}}, ProviderID: "P"},
		{Text: "done", ProviderID: "P"},
	}}
	tools := &fakeTools{}
	w := New(Deps{
		Config: config.Config{LLMStub: false, PromptsRoot: root},
		Store:  store, Locks: &fakeLock{}, Interrupt: &fakeInterrupt{},
		Tools: tools, LLM: llm,
	})
	w.process(context.Background(), sampleJob())
	if len(tools.calls) != 1 || tools.calls[0].Name != "docs_search" {
		t.Fatalf("calls=%#v", tools.calls)
	}
	got := typesOf(store)
	if !containsType(got, enum.ItemToolCall) || !containsType(got, enum.ItemToolResult) || !containsType(got, enum.ItemAgentMessage) {
		t.Fatalf("got=%v", got)
	}
}

func TestRunAgentToolExecuteErrorAndDedupeStop(t *testing.T) {
	root := t.TempDir()
	mustWritePrompt(t, root)
	store := &fakeStore{}
	same := []service.ToolCall{{CallID: "c1", Name: "list_dir", Arguments: map[string]any{"path": ""}}}
	llm := &scriptLLM{resps: []service.LLMResult{
		{ToolCalls: same},
		{ToolCalls: same},
		{ToolCalls: same},
	}}
	tools := &fakeTools{exec: func(service.ToolCall) (service.ToolEnvelope, error) {
		return service.ToolEnvelope{}, errors.New("boom")
	}}
	w := New(Deps{
		Config: config.Config{LLMStub: false, PromptsRoot: root, MaxToolRounds: 8},
		Store:  store, Locks: &fakeLock{}, Interrupt: &fakeInterrupt{},
		Tools: tools, LLM: llm,
	})
	w.process(context.Background(), sampleJob())
	got := typesOf(store)
	if !containsType(got, enum.ItemAgentMessage) || !containsType(got, enum.ItemTurnCompleted) {
		t.Fatalf("got=%v", got)
	}
	store.mu.Lock()
	var agentText string
	for _, it := range store.items {
		if it.Type == enum.ItemAgentMessage {
			agentText, _ = it.Payload["text"].(string)
		}
	}
	store.mu.Unlock()
	if !strings.Contains(agentText, "repeated identical tool") {
		t.Fatalf("agentText=%q", agentText)
	}
	// tool result should still be written with envelope error from execErr path
	store.mu.Lock()
	var sawErr bool
	for _, it := range store.items {
		if it.Type == enum.ItemToolResult {
			env, _ := it.Payload["envelope"].(map[string]any)
			if env != nil && env["ok"] == false {
				sawErr = true
			}
		}
	}
	store.mu.Unlock()
	if !sawErr {
		t.Fatal("expected tool_result with ok=false")
	}
}

func TestRunAgentInterruptMidLLM(t *testing.T) {
	root := t.TempDir()
	mustWritePrompt(t, root)
	store := &fakeStore{}
	intr := &toggleInterrupt{}
	w := New(Deps{
		Config: config.Config{LLMStub: false, PromptsRoot: root},
		Store:  store, Locks: &fakeLock{}, Interrupt: intr,
		Tools: &fakeTools{}, LLM: &blockingLLM{blockFor: 10 * time.Second},
	})

	// Start the worker; after a short delay set the interrupt flag to simulate Stop.
	go func() {
		time.Sleep(50 * time.Millisecond)
		intr.SetRequested(true)
	}()
	w.process(context.Background(), sampleJob())

	if !containsType(typesOf(store), enum.ItemTurnFailed) {
		t.Fatalf("expected turn failed, got=%v", typesOf(store))
	}
	store.mu.Lock()
	var code string
	for _, it := range store.items {
		if it.Type == enum.ItemTurnFailed {
			if errObj, ok := it.Payload["error"].(map[string]any); ok {
				code, _ = errObj["code"].(string)
			}
		}
	}
	store.mu.Unlock()
	if code != "interrupted" {
		t.Fatalf("expected interrupted code, got=%q", code)
	}
}

func TestRunAgentInterruptAndMissingDeps(t *testing.T) {
	root := t.TempDir()
	mustWritePrompt(t, root)
	store := &fakeStore{}
	w := New(Deps{
		Config: config.Config{LLMStub: false, PromptsRoot: root},
		Store:  store, Locks: &fakeLock{}, Interrupt: &fakeInterrupt{requested: true},
		Tools: &fakeTools{}, LLM: &scriptLLM{resps: []service.LLMResult{{Text: "nope"}}},
	})
	w.process(context.Background(), sampleJob())
	if !containsType(typesOf(store), enum.ItemTurnFailed) {
		t.Fatalf("got=%v", typesOf(store))
	}

	store2 := &fakeStore{}
	w2 := New(Deps{
		Config: config.Config{LLMStub: false},
		Store:  store2, Locks: &fakeLock{}, Interrupt: &fakeInterrupt{},
	})
	w2.process(context.Background(), sampleJob())
	if !containsType(typesOf(store2), enum.ItemTurnFailed) {
		t.Fatalf("got=%v", typesOf(store2))
	}
}

func TestRunAgentMaxRoundsToolOnly(t *testing.T) {
	root := t.TempDir()
	mustWritePrompt(t, root)
	store := &fakeStore{}
	llm := &scriptLLM{resps: []service.LLMResult{
		{ToolCalls: []service.ToolCall{{Name: "a", Arguments: map[string]any{"i": 1}}}},
		{ToolCalls: []service.ToolCall{{Name: "b", Arguments: map[string]any{"i": 2}}}},
	}}
	w := New(Deps{
		Config: config.Config{LLMStub: false, PromptsRoot: root, MaxToolRounds: 2},
		Store:  store, Locks: &fakeLock{}, Interrupt: &fakeInterrupt{},
		Tools: &fakeTools{}, LLM: llm,
	})
	w.process(context.Background(), sampleJob())
	store.mu.Lock()
	var text string
	for _, it := range store.items {
		if it.Type == enum.ItemAgentMessage {
			text, _ = it.Payload["text"].(string)
		}
	}
	store.mu.Unlock()
	if !strings.Contains(text, "max tool rounds") {
		t.Fatalf("text=%q", text)
	}
}

func TestRunAgentLLMErrorAndEmptyText(t *testing.T) {
	root := t.TempDir()
	mustWritePrompt(t, root)
	store := &fakeStore{}
	w := New(Deps{
		Config: config.Config{LLMStub: false, PromptsRoot: root},
		Store:  store, Locks: &fakeLock{}, Interrupt: &fakeInterrupt{},
		Tools: &fakeTools{}, LLM: &scriptLLM{errs: []error{errors.New("upstream")}},
	})
	w.process(context.Background(), sampleJob())
	if !containsType(typesOf(store), enum.ItemTurnFailed) {
		t.Fatalf("got=%v", typesOf(store))
	}

	// Two empty rounds: first nudges, second still empty → runtime placeholder + warn path.
	store2 := &fakeStore{}
	w2 := New(Deps{
		Config: config.Config{LLMStub: false, PromptsRoot: root},
		Store:  store2, Locks: &fakeLock{}, Interrupt: &fakeInterrupt{},
		Tools: &fakeTools{}, LLM: &scriptLLM{resps: []service.LLMResult{
			{Text: "   ", Reasoning: "planning…", Model: service.ModelRef{Provider: "P", ID: "m", API: "responses"}, Status: "completed"},
			{Text: "", Reasoning: "still thinking", Model: service.ModelRef{Provider: "P", ID: "m", API: "responses"}, Status: "completed"},
		}},
	})
	w2.process(context.Background(), sampleJob())
	store2.mu.Lock()
	var text string
	var origin string
	var agentCount int
	var reasonCount int
	for _, it := range store2.items {
		switch it.Type {
		case enum.ItemAgentMessage:
			agentCount++
			text, _ = it.Payload["text"].(string)
			origin, _ = it.Payload["origin"].(string)
		case enum.ItemReasoning:
			reasonCount++
		}
	}
	store2.mu.Unlock()
	if text != "(empty model response)" {
		t.Fatalf("text=%q", text)
	}
	if origin != "runtime" {
		t.Fatalf("origin=%q", origin)
	}
	if agentCount != 1 {
		t.Fatalf("agentCount=%d", agentCount)
	}
	if reasonCount < 1 {
		t.Fatalf("expected reasoning items, got %d", reasonCount)
	}
}

func TestEmptyModelResponseNudgeRecovers(t *testing.T) {
	root := t.TempDir()
	mustWritePrompt(t, root)
	store := &fakeStore{}
	llm := &scriptLLM{resps: []service.LLMResult{
		{Text: "", Reasoning: "will answer next"},
		{Text: "Jawaban setelah nudge"},
	}}
	w := New(Deps{
		Config: config.Config{LLMStub: false, PromptsRoot: root},
		Store:  store, Locks: &fakeLock{}, Interrupt: &fakeInterrupt{},
		Tools: &fakeTools{}, LLM: llm,
	})
	w.process(context.Background(), sampleJob())
	store.mu.Lock()
	var text string
	for _, it := range store.items {
		if it.Type == enum.ItemAgentMessage {
			text, _ = it.Payload["text"].(string)
		}
	}
	calls := llm.calls
	store.mu.Unlock()
	if text != "Jawaban setelah nudge" {
		t.Fatalf("text=%q", text)
	}
	if calls != 2 {
		t.Fatalf("expected 2 LLM rounds (empty+nudge), got %d", calls)
	}
}

func TestMaybeAutoTitleSkipsManual(t *testing.T) {
	job := sampleJob()
	title, _ := valueobject.NewTitle("Manual")
	store := &fakeStore{meta: map[string]entity.ConversationMeta{
		job.ThreadID.String(): {
			ThreadID: job.ThreadID, Title: &title, TitleSource: enum.TitleManual,
		},
	}}
	w := New(Deps{Store: store})
	w.maybeAutoTitle(context.Background(), job)
	if store.meta[job.ThreadID.String()].TitleSource != enum.TitleManual {
		t.Fatal("should not overwrite manual title")
	}
	store.metaErr = errors.New("x")
	w.maybeAutoTitle(context.Background(), job) // no panic
}

func TestTurnCompletedIgnoresOtherTurns(t *testing.T) {
	job := sampleJob()
	other, _ := valueobject.NewTurnID("trn_other")
	store := &fakeStore{items: []entity.TranscriptItem{
		{Seq: 1, ThreadID: job.ThreadID, TurnID: other, Type: enum.ItemTurnCompleted},
		{Seq: 2, ThreadID: job.ThreadID, TurnID: job.TurnID, Type: enum.ItemAgentMessage},
	}}
	w := New(Deps{Store: store})
	if w.turnCompleted(context.Background(), job) {
		t.Fatal("other turn completed should not count")
	}
}

func TestFindTurnAndTurnCompleted(t *testing.T) {
	job := sampleJob()
	store := &fakeStore{items: []entity.TranscriptItem{
		{Seq: 1, ThreadID: job.ThreadID, TurnID: job.TurnID, Type: enum.ItemUserMessage, Payload: map[string]any{"text": "x"}},
		{Seq: 2, ThreadID: job.ThreadID, TurnID: job.TurnID, Type: enum.ItemTurnFailed},
	}}
	w := New(Deps{Store: store})
	text, ok := w.findTurnUserText(context.Background(), job)
	if !ok || text != "x" {
		t.Fatalf("%v %q", ok, text)
	}
	if w.turnCompleted(context.Background(), job) {
		t.Fatal("failed turn should not be completed")
	}
	store.items = append(store.items, entity.TranscriptItem{
		Seq: 3, ThreadID: job.ThreadID, TurnID: job.TurnID, Type: enum.ItemTurnCompleted,
	})
	if !w.turnCompleted(context.Background(), job) {
		t.Fatal("want completed")
	}
	store.getErr = errors.New("nope")
	if _, ok := w.findTurnUserText(context.Background(), job); ok {
		t.Fatal("want false on get error")
	}
	if w.turnCompleted(context.Background(), job) {
		t.Fatal("want false on get error")
	}
}

func TestProcessPanicRecovery(t *testing.T) {
	store := &fakeStore{
		appendFn: func(it entity.TranscriptItem) error {
			if it.Type == enum.ItemUserMessage {
				panic("kaboom")
			}
			return nil
		},
	}
	lock := &fakeLock{}
	w := New(Deps{
		Config: config.Config{LLMStub: true},
		Store:  store, Locks: lock, Interrupt: &fakeInterrupt{},
	})
	w.process(context.Background(), sampleJob())
	if !containsType(typesOf(store), enum.ItemTurnFailed) {
		t.Fatalf("got=%v", typesOf(store))
	}
	if store.cleared == 0 || len(lock.released) == 0 {
		t.Fatal("cleanup should run after panic")
	}
}

func mustWritePrompt(t *testing.T, root string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "system.md"), []byte("sys static"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "developer.md"), []byte("dev tools={{available_tools}}"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func containsType(types []enum.ItemType, want enum.ItemType) bool {
	for _, t := range types {
		if t == want {
			return true
		}
	}
	return false
}
