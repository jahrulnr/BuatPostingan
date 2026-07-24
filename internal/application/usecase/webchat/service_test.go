package webchat_test

import (
	"context"
	"testing"

	"buatpostingan/internal/usecase/webchat"
	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/enum"
	"buatpostingan/internal/domain/service"
	"buatpostingan/internal/domain/valueobject"
	"buatpostingan/internal/pkg/apperr"
)

type memDeps struct {
	threads   *memStore
	locks     *memLock
	interrupt *memInterrupt
	floor     *memFloor
	rate      *memRate
	docs      *memDocs
	worker    *memWorker
	events    *memEvents
}

func newMemDeps() memDeps {
	return memDeps{
		threads:   &memStore{threads: map[string]*entity.ThreadSnapshot{}},
		locks:     &memLock{},
		interrupt: &memInterrupt{},
		floor:     &memFloor{},
		rate:      &memRate{},
		docs:      &memDocs{usable: true},
		worker:    &memWorker{},
		events:    &memEvents{},
	}
}

func (d memDeps) service() *webchat.Service {
	return webchat.NewService(webchat.Deps{
		Threads:   d.threads,
		Locks:     d.locks,
		Interrupt: d.interrupt,
		Floor:     d.floor,
		RateLimit: d.rate,
		Redactor:  identityRedactor{},
		Docs:      d.docs,
		Events:    d.events,
		Worker:    d.worker,
	})
}

func TestStartTurnOrder_HappyPath(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	tid, _ := valueobject.NewThreadID("thr_1")
	d.threads.threads[tid.String()] = &entity.ThreadSnapshot{ThreadID: tid, SeqHead: 2}

	uc := d.service()
	out, err := uc.StartTurn(context.Background(), webchat.StartTurnInput{
		ThreadID:    tid,
		Message:     "hello",
		AdminUserID: 7,
		AdminName:   "Admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != "queued" || out.SeqHead != 2 {
		t.Fatalf("unexpected: %+v", out)
	}
	if !d.worker.called {
		t.Fatal("expected worker enqueue")
	}
	if d.worker.job.IsRetry || d.worker.job.Message != "hello" {
		t.Fatalf("job: %+v", d.worker.job)
	}
	if d.worker.job.LockToken == "" {
		t.Fatal("expected lock token on job")
	}
}

func TestStartTurn_DocsNotReady(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	d.docs.usable = false
	tid, _ := valueobject.NewThreadID("thr_1")
	d.threads.threads[tid.String()] = &entity.ThreadSnapshot{ThreadID: tid}

	_, err := d.service().StartTurn(context.Background(), webchat.StartTurnInput{
		ThreadID: tid, Message: "x", AdminUserID: 1,
	})
	ae, ok := apperr.As(err)
	if !ok || ae.Code != apperr.CodeDocsIndexNotReady {
		t.Fatalf("got %v", err)
	}
}

func TestStartTurn_Busy(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	d.locks.busy = true
	tid, _ := valueobject.NewThreadID("thr_1")
	d.threads.threads[tid.String()] = &entity.ThreadSnapshot{ThreadID: tid}

	_, err := d.service().StartTurn(context.Background(), webchat.StartTurnInput{
		ThreadID: tid, Message: "x", AdminUserID: 1,
	})
	ae, ok := apperr.As(err)
	if !ok || ae.Code != apperr.CodeThreadBusy {
		t.Fatalf("got %v", err)
	}
}

func TestRetryTurn_NotRetryableInterrupted(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	tid, _ := valueobject.NewThreadID("thr_1")
	trn, _ := valueobject.NewTurnID("trn_1")
	d.threads.threads[tid.String()] = &entity.ThreadSnapshot{
		ThreadID: tid,
		SeqHead:  3,
		Items: []entity.TranscriptItem{
			{Type: enum.ItemUserMessage, TurnID: trn, Payload: map[string]any{"text": "hi", "admin_user_id": int64(1)}},
			{Type: enum.ItemTurnFailed, TurnID: trn, Payload: map[string]any{"error": map[string]any{"code": "interrupted"}}},
		},
	}
	_, err := d.service().RetryTurn(context.Background(), tid, trn, 1)
	ae, ok := apperr.As(err)
	if !ok || ae.Code != apperr.CodeNotRetryable {
		t.Fatalf("got %v", err)
	}
}

func TestRetryTurn_Happy(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	tid, _ := valueobject.NewThreadID("thr_1")
	trn, _ := valueobject.NewTurnID("trn_1")
	d.threads.threads[tid.String()] = &entity.ThreadSnapshot{
		ThreadID: tid,
		SeqHead:  4,
		Items: []entity.TranscriptItem{
			{Type: enum.ItemUserMessage, TurnID: trn, Payload: map[string]any{"text": "hi", "admin_user_id": int64(1)}},
			{Type: enum.ItemTurnFailed, TurnID: trn, Payload: map[string]any{"error": map[string]any{"code": "llm_error"}}},
		},
	}
	out, err := d.service().RetryTurn(context.Background(), tid, trn, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !d.worker.job.IsRetry || d.worker.job.Message != "hi" {
		t.Fatalf("job %+v", d.worker.job)
	}
	if out.TurnID != trn {
		t.Fatalf("turn %s", out.TurnID)
	}
}

// --- fakes ---

type identityRedactor struct{}

func (identityRedactor) Redact(_ context.Context, text string) (string, error) { return text, nil }

type memStore struct {
	threads map[string]*entity.ThreadSnapshot
}

func (m *memStore) CreateThread(_ context.Context, admin int64) (entity.ThreadSnapshot, error) {
	tid, _ := valueobject.NewThreadID("thr_new")
	snap := entity.ThreadSnapshot{ThreadID: tid, SeqHead: 1}
	m.threads[tid.String()] = &snap
	_ = admin
	return snap, nil
}
func (m *memStore) GetThread(_ context.Context, id valueobject.ThreadID, _ uint64) (entity.ThreadSnapshot, error) {
	s, ok := m.threads[id.String()]
	if !ok {
		return entity.ThreadSnapshot{}, apperr.NotFound("thread not found")
	}
	cp := *s
	return cp, nil
}
func (m *memStore) AppendItem(context.Context, valueobject.ThreadID, entity.TranscriptItem) (entity.TranscriptItem, error) {
	return entity.TranscriptItem{}, nil
}
func (m *memStore) ListConversations(context.Context) ([]entity.ConversationMeta, error) {
	return nil, nil
}
func (m *memStore) RenameThread(context.Context, valueobject.ThreadID, valueobject.Title) error {
	return nil
}
func (m *memStore) SoftDeleteThread(context.Context, valueobject.ThreadID) error { return nil }
func (m *memStore) SeqHead(_ context.Context, id valueobject.ThreadID) (uint64, error) {
	s, ok := m.threads[id.String()]
	if !ok {
		return 0, apperr.NotFound("thread not found")
	}
	return s.SeqHead, nil
}
func (m *memStore) ResolveConversation(_ context.Context, id valueobject.ThreadID) (entity.ConversationMeta, bool, error) {
	_, ok := m.threads[id.String()]
	if !ok {
		return entity.ConversationMeta{}, false, nil
	}
	return entity.ConversationMeta{ThreadID: id}, true, nil
}
func (m *memStore) AppendConversationMeta(context.Context, entity.ConversationMeta) error {
	return nil
}
func (m *memStore) ClearActiveTurn(context.Context, valueobject.ThreadID) error {
	return nil
}

type memLock struct{ busy bool }

func (m *memLock) TryAcquire(context.Context, valueobject.ThreadID) (string, error) {
	if m.busy {
		return "", apperr.ThreadBusy()
	}
	return "tok", nil
}
func (m *memLock) Release(context.Context, valueobject.ThreadID, string) error { return nil }
func (m *memLock) IsBusy(context.Context, valueobject.ThreadID) (bool, error)  { return m.busy, nil }

type memInterrupt struct{}

func (memInterrupt) Request(context.Context, valueobject.ThreadID, valueobject.TurnID) error {
	return nil
}
func (memInterrupt) IsRequested(context.Context, valueobject.ThreadID, valueobject.TurnID) (bool, error) {
	return false, nil
}
func (memInterrupt) Clear(context.Context, valueobject.ThreadID, valueobject.TurnID) error {
	return nil
}

type memFloor struct{}

func (memFloor) Assert(context.Context, valueobject.ThreadID, int64) error { return nil }
func (memFloor) Acquire(context.Context, valueobject.ThreadID, int64, valueobject.TurnID) error {
	return nil
}
func (memFloor) Remaining(context.Context, valueobject.ThreadID) (*int64, int, error) {
	return nil, 0, nil
}

type memRate struct{}

func (memRate) Assert(context.Context, int64) (int, error) { return 0, nil }

type memDocs struct{ usable bool }

func (m *memDocs) Gate(context.Context) (entity.DocsIndexGate, error) {
	return entity.DocsIndexGate{Usable: m.usable, Status: "ready"}, nil
}
func (m *memDocs) Search(context.Context, string, int) (any, error) { return nil, nil }
func (m *memDocs) Reindex(context.Context) error                    { return nil }

type memWorker struct {
	called bool
	job    service.TurnJob
}

func (m *memWorker) Enqueue(_ context.Context, job service.TurnJob) error {
	m.called = true
	m.job = job
	return nil
}

type memEvents struct{}

func (memEvents) Subscribe(context.Context, valueobject.ThreadID, uint64, service.EventEmitFn) error {
	return nil
}
