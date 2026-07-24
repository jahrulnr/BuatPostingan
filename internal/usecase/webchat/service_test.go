package webchat_test

import (
	"context"
	"errors"
	"testing"

	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/enum"
	"buatpostingan/internal/domain/service"
	"buatpostingan/internal/domain/valueobject"
	"buatpostingan/internal/pkg/apperr"
	"buatpostingan/internal/usecase/webchat"
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
	redactor  service.SecretRedactor
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
		redactor:  identityRedactor{},
	}
}

func (d memDeps) service() *webchat.Service {
	return webchat.NewService(webchat.Deps{
		Threads:   d.threads,
		Locks:     d.locks,
		Interrupt: d.interrupt,
		Floor:     d.floor,
		RateLimit: d.rate,
		Redactor:  d.redactor,
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
		Message:     "  hello  ",
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
	if out.FloorHolderAdminID == nil || *out.FloorHolderAdminID != 7 {
		t.Fatalf("floor holder: %+v", out.FloorHolderAdminID)
	}
}

func TestStartTurn_EmptyMessage(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	tid, _ := valueobject.NewThreadID("thr_1")
	_, err := d.service().StartTurn(context.Background(), webchat.StartTurnInput{
		ThreadID: tid, Message: "   ", AdminUserID: 1,
	})
	ae, ok := apperr.As(err)
	if !ok || ae.Code != apperr.CodeEmpty {
		t.Fatalf("got %v", err)
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

func TestStartTurn_DocsGateError(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	d.docs.err = errors.New("gate boom")
	tid, _ := valueobject.NewThreadID("thr_1")
	d.threads.threads[tid.String()] = &entity.ThreadSnapshot{ThreadID: tid}
	_, err := d.service().StartTurn(context.Background(), webchat.StartTurnInput{
		ThreadID: tid, Message: "x", AdminUserID: 1,
	})
	if err == nil || err.Error() != "gate boom" {
		t.Fatalf("got %v", err)
	}
}

func TestStartTurn_ThreadNotFound(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	tid, _ := valueobject.NewThreadID("thr_missing")
	_, err := d.service().StartTurn(context.Background(), webchat.StartTurnInput{
		ThreadID: tid, Message: "x", AdminUserID: 1,
	})
	ae, ok := apperr.As(err)
	if !ok || ae.Code != apperr.CodeNotFound {
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

func TestStartTurn_RateLimitedRemap(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	d.rate.err = apperr.RateLimited(0) // retryAfter from Assert drives remap
	d.rate.retryAfter = 42
	tid, _ := valueobject.NewThreadID("thr_1")
	d.threads.threads[tid.String()] = &entity.ThreadSnapshot{ThreadID: tid}

	_, err := d.service().StartTurn(context.Background(), webchat.StartTurnInput{
		ThreadID: tid, Message: "x", AdminUserID: 1,
	})
	ae, ok := apperr.As(err)
	if !ok || ae.Code != apperr.CodeRateLimited {
		t.Fatalf("got %v", err)
	}
	if ae.Extra["retry_after"] != 42 {
		t.Fatalf("extra %+v", ae.Extra)
	}
}

func TestStartTurn_RateLimitOtherError(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	d.rate.err = errors.New("rate store down")
	tid, _ := valueobject.NewThreadID("thr_1")
	d.threads.threads[tid.String()] = &entity.ThreadSnapshot{ThreadID: tid}
	_, err := d.service().StartTurn(context.Background(), webchat.StartTurnInput{
		ThreadID: tid, Message: "x", AdminUserID: 1,
	})
	if err == nil || err.Error() != "rate store down" {
		t.Fatalf("got %v", err)
	}
}

func TestStartTurn_FloorLocked(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	d.floor.assertErr = apperr.FloorLocked(99, 10)
	tid, _ := valueobject.NewThreadID("thr_1")
	d.threads.threads[tid.String()] = &entity.ThreadSnapshot{ThreadID: tid}
	_, err := d.service().StartTurn(context.Background(), webchat.StartTurnInput{
		ThreadID: tid, Message: "x", AdminUserID: 1,
	})
	ae, ok := apperr.As(err)
	if !ok || ae.Code != apperr.CodeFloorLocked {
		t.Fatalf("got %v", err)
	}
}

func TestStartTurn_RedactError(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	d.redactor = errRedactor{err: errors.New("redact fail")}
	tid, _ := valueobject.NewThreadID("thr_1")
	d.threads.threads[tid.String()] = &entity.ThreadSnapshot{ThreadID: tid}
	_, err := d.service().StartTurn(context.Background(), webchat.StartTurnInput{
		ThreadID: tid, Message: "secret", AdminUserID: 1,
	})
	if err == nil || err.Error() != "redact fail" {
		t.Fatalf("got %v", err)
	}
}

func TestStartTurn_NilRedactor(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	d.redactor = nil
	tid, _ := valueobject.NewThreadID("thr_1")
	d.threads.threads[tid.String()] = &entity.ThreadSnapshot{ThreadID: tid, SeqHead: 1}
	out, err := d.service().StartTurn(context.Background(), webchat.StartTurnInput{
		ThreadID: tid, Message: "plain", AdminUserID: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if d.worker.job.Message != "plain" || out.Status != "queued" {
		t.Fatalf("job %+v out %+v", d.worker.job, out)
	}
}

func TestStartTurn_FloorAcquireReleasesLock(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	d.floor.acquireErr = errors.New("acquire fail")
	tid, _ := valueobject.NewThreadID("thr_1")
	d.threads.threads[tid.String()] = &entity.ThreadSnapshot{ThreadID: tid}
	_, err := d.service().StartTurn(context.Background(), webchat.StartTurnInput{
		ThreadID: tid, Message: "x", AdminUserID: 1,
	})
	if err == nil || err.Error() != "acquire fail" {
		t.Fatalf("got %v", err)
	}
	if !d.locks.released {
		t.Fatal("expected lock release after acquire fail")
	}
}

func TestStartTurn_SeqHeadErrorReleasesLock(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	d.threads.seqHeadErr = errors.New("seq boom")
	tid, _ := valueobject.NewThreadID("thr_1")
	d.threads.threads[tid.String()] = &entity.ThreadSnapshot{ThreadID: tid}
	_, err := d.service().StartTurn(context.Background(), webchat.StartTurnInput{
		ThreadID: tid, Message: "x", AdminUserID: 1,
	})
	if err == nil || err.Error() != "seq boom" {
		t.Fatalf("got %v", err)
	}
	if !d.locks.released {
		t.Fatal("expected lock release")
	}
}

func TestStartTurn_EnqueueErrorReleasesLock(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	d.worker.err = errors.New("queue full")
	tid, _ := valueobject.NewThreadID("thr_1")
	d.threads.threads[tid.String()] = &entity.ThreadSnapshot{ThreadID: tid, SeqHead: 1}
	_, err := d.service().StartTurn(context.Background(), webchat.StartTurnInput{
		ThreadID: tid, Message: "x", AdminUserID: 1,
	})
	if err == nil || err.Error() != "queue full" {
		t.Fatalf("got %v", err)
	}
	if !d.locks.released {
		t.Fatal("expected lock release")
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

func TestRetryTurn_CompletedNotRetryable(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	tid, _ := valueobject.NewThreadID("thr_1")
	trn, _ := valueobject.NewTurnID("trn_1")
	d.threads.threads[tid.String()] = &entity.ThreadSnapshot{
		ThreadID: tid,
		Items: []entity.TranscriptItem{
			{Type: enum.ItemUserMessage, TurnID: trn, Payload: map[string]any{"text": "hi", "admin_user_id": int64(1)}},
			{Type: enum.ItemTurnFailed, TurnID: trn, Payload: map[string]any{"error": map[string]any{"code": "llm_error"}}},
			{Type: enum.ItemTurnCompleted, TurnID: trn},
		},
	}
	_, err := d.service().RetryTurn(context.Background(), tid, trn, 1)
	ae, ok := apperr.As(err)
	if !ok || ae.Code != apperr.CodeNotRetryable {
		t.Fatalf("got %v", err)
	}
}

func TestRetryTurn_UserMessageMissing(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	tid, _ := valueobject.NewThreadID("thr_1")
	trn, _ := valueobject.NewTurnID("trn_1")
	d.threads.threads[tid.String()] = &entity.ThreadSnapshot{ThreadID: tid, Items: nil}
	_, err := d.service().RetryTurn(context.Background(), tid, trn, 1)
	ae, ok := apperr.As(err)
	if !ok || ae.Code != apperr.CodeNotFound {
		t.Fatalf("got %v", err)
	}
}

func TestRetryTurn_NotInitiator(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	tid, _ := valueobject.NewThreadID("thr_1")
	trn, _ := valueobject.NewTurnID("trn_1")
	d.threads.threads[tid.String()] = &entity.ThreadSnapshot{
		ThreadID: tid,
		Items: []entity.TranscriptItem{
			{Type: enum.ItemUserMessage, TurnID: trn, Payload: map[string]any{"text": "hi", "admin_user_id": float64(2)}},
			{Type: enum.ItemTurnFailed, TurnID: trn, Payload: map[string]any{"error": map[string]any{"code": "llm_error"}}},
		},
	}
	_, err := d.service().RetryTurn(context.Background(), tid, trn, 1)
	ae, ok := apperr.As(err)
	if !ok || ae.Code != apperr.CodeNotInitiator {
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
			{Type: enum.ItemUserMessage, TurnID: trn, Payload: map[string]any{
				"text": "hi", "admin_user_id": int(1), "admin_display_name": "Ada",
			}},
			{Type: enum.ItemTurnFailed, TurnID: trn, Payload: map[string]any{"error": map[string]any{"code": "llm_error"}}},
		},
	}
	out, err := d.service().RetryTurn(context.Background(), tid, trn, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !d.worker.job.IsRetry || d.worker.job.Message != "hi" || d.worker.job.AdminName != "Ada" {
		t.Fatalf("job %+v", d.worker.job)
	}
	if out.TurnID != trn {
		t.Fatalf("turn %s", out.TurnID)
	}
}

func TestRetryTurn_FloorAcquireFail(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	d.floor.acquireErr = errors.New("no floor")
	tid, _ := valueobject.NewThreadID("thr_1")
	trn, _ := valueobject.NewTurnID("trn_1")
	d.threads.threads[tid.String()] = &entity.ThreadSnapshot{
		ThreadID: tid,
		Items: []entity.TranscriptItem{
			{Type: enum.ItemUserMessage, TurnID: trn, Payload: map[string]any{"text": "hi", "admin_user_id": int64(1)}},
			{Type: enum.ItemTurnFailed, TurnID: trn, Payload: map[string]any{"error": map[string]any{"code": "x"}}},
		},
	}
	_, err := d.service().RetryTurn(context.Background(), tid, trn, 1)
	if err == nil || err.Error() != "no floor" {
		t.Fatalf("got %v", err)
	}
	if !d.locks.released {
		t.Fatal("expected release")
	}
}

func TestRetryTurn_EnqueueFail(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	d.worker.err = errors.New("enqueue")
	tid, _ := valueobject.NewThreadID("thr_1")
	trn, _ := valueobject.NewTurnID("trn_1")
	d.threads.threads[tid.String()] = &entity.ThreadSnapshot{
		ThreadID: tid,
		Items: []entity.TranscriptItem{
			{Type: enum.ItemUserMessage, TurnID: trn, Payload: map[string]any{"text": "hi", "admin_user_id": int64(1)}},
			{Type: enum.ItemTurnFailed, TurnID: trn, Payload: map[string]any{"error": map[string]any{"code": "x"}}},
		},
	}
	_, err := d.service().RetryTurn(context.Background(), tid, trn, 1)
	if err == nil || err.Error() != "enqueue" {
		t.Fatalf("got %v", err)
	}
	if !d.locks.released {
		t.Fatal("expected release")
	}
}

func TestInterruptTurn_HappyAndActiveFallback(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	tid, _ := valueobject.NewThreadID("thr_1")
	trn, _ := valueobject.NewTurnID("trn_active")
	admin := int64(5)
	d.threads.threads[tid.String()] = &entity.ThreadSnapshot{
		ThreadID:                    tid,
		ActiveTurnID:                &trn,
		ActiveTurnInitiatorAdminID:  &admin,
	}
	var blank valueobject.TurnID
	if err := d.service().InterruptTurn(context.Background(), tid, blank, 5); err != nil {
		t.Fatal(err)
	}
	if d.interrupt.threadID != tid || d.interrupt.turnID != trn {
		t.Fatalf("interrupt %+v %+v", d.interrupt.threadID, d.interrupt.turnID)
	}
}

func TestInterruptTurn_NotInitiator(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	tid, _ := valueobject.NewThreadID("thr_1")
	trn, _ := valueobject.NewTurnID("trn_1")
	admin := int64(5)
	d.threads.threads[tid.String()] = &entity.ThreadSnapshot{
		ThreadID:                   tid,
		ActiveTurnID:               &trn,
		ActiveTurnInitiatorAdminID: &admin,
	}
	err := d.service().InterruptTurn(context.Background(), tid, trn, 9)
	ae, ok := apperr.As(err)
	if !ok || ae.Code != apperr.CodeNotInitiator {
		t.Fatalf("got %v", err)
	}
}

func TestInterruptTurn_TurnIDRequired(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	tid, _ := valueobject.NewThreadID("thr_1")
	d.threads.threads[tid.String()] = &entity.ThreadSnapshot{ThreadID: tid}
	var blank valueobject.TurnID
	err := d.service().InterruptTurn(context.Background(), tid, blank, 1)
	ae, ok := apperr.As(err)
	if !ok || ae.Code != apperr.CodeValidation {
		t.Fatalf("got %v", err)
	}
}

func TestListConversations_FloorViews(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	tidA, _ := valueobject.NewThreadID("thr_a")
	tidB, _ := valueobject.NewThreadID("thr_b")
	holder := int64(3)
	titleA, _ := valueobject.NewTitle("A")
	titleB, _ := valueobject.NewTitle("B")
	d.threads.list = []entity.ConversationMeta{
		{ThreadID: tidA, Title: &titleA},
		{ThreadID: tidB, Title: &titleB, FloorHolderAdminID: &holder},
	}
	d.floor.remainingBy = map[string]struct {
		holder *int64
		sec    int
		err    error
	}{
		"thr_a": {holder: &holder, sec: 30},
		"thr_b": {sec: 0}, // expired → clear holder
	}
	out, err := d.service().ListConversations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !out.DocsIndex.Usable || len(out.Conversations) != 2 {
		t.Fatalf("%+v", out)
	}
	if out.Conversations[0].FloorRemainingSec != 30 || out.Conversations[0].Meta.FloorHolderAdminID == nil {
		t.Fatalf("a: %+v", out.Conversations[0])
	}
	if out.Conversations[1].Meta.FloorHolderAdminID != nil {
		t.Fatalf("b holder should clear: %+v", out.Conversations[1])
	}
}

func TestListConversations_ListErrorAndFloorErr(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	d.threads.listErr = errors.New("list fail")
	_, err := d.service().ListConversations(context.Background())
	if err == nil || err.Error() != "list fail" {
		t.Fatalf("got %v", err)
	}
	d.threads.listErr = nil
	tid, _ := valueobject.NewThreadID("thr_z")
	d.threads.list = []entity.ConversationMeta{{ThreadID: tid}}
	d.floor.remainingBy = map[string]struct {
		holder *int64
		sec    int
		err    error
	}{"thr_z": {err: errors.New("floor")}}
	out, err := d.service().ListConversations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if out.Conversations[0].FloorRemainingSec != 0 || out.Conversations[0].Meta.FloorHolderAdminID != nil {
		t.Fatalf("%+v", out.Conversations[0])
	}
}

func TestCreateThread_StoreError(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	d.threads.createErr = errors.New("create fail")
	_, err := d.service().CreateThread(context.Background(), 1)
	if err == nil || err.Error() != "create fail" {
		t.Fatalf("got %v", err)
	}
}

func TestGetThread_Errors(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	missing, _ := valueobject.NewThreadID("thr_x")
	_, err := d.service().GetThread(context.Background(), missing, 0)
	ae, ok := apperr.As(err)
	if !ok || ae.Code != apperr.CodeNotFound {
		t.Fatalf("got %v", err)
	}
	tid, _ := valueobject.NewThreadID("thr_1")
	d.threads.threads[tid.String()] = &entity.ThreadSnapshot{ThreadID: tid}
	d.floor.remainingBy = map[string]struct {
		holder *int64
		sec    int
		err    error
	}{"thr_1": {err: errors.New("floor")}}
	snap, err := d.service().GetThread(context.Background(), tid, 0)
	if err != nil {
		t.Fatal(err)
	}
	if snap.FloorHolderAdminID != nil || snap.FloorRemainingSec != 0 {
		t.Fatalf("floor err should skip: %+v", snap)
	}
}

func TestRenameThread_StoreError(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	tid, _ := valueobject.NewThreadID("thr_1")
	d.threads.threads[tid.String()] = &entity.ThreadSnapshot{ThreadID: tid}
	d.threads.renameErr = errors.New("rename fail")
	title, _ := valueobject.NewTitle("X")
	_, err := d.service().RenameThread(context.Background(), tid, title)
	if err == nil || err.Error() != "rename fail" {
		t.Fatalf("got %v", err)
	}
}

func TestRetryTurn_DocsBusyFloorRate(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	tid, _ := valueobject.NewThreadID("thr_1")
	trn, _ := valueobject.NewTurnID("trn_1")
	items := []entity.TranscriptItem{
		{Type: enum.ItemUserMessage, TurnID: trn, Payload: map[string]any{"text": "hi", "admin_user_id": "skip"}},
		{Type: enum.ItemTurnFailed, TurnID: trn, Payload: map[string]any{"error": "plain"}},
	}
	d.threads.threads[tid.String()] = &entity.ThreadSnapshot{ThreadID: tid, Items: items}

	d.docs.usable = false
	_, err := d.service().RetryTurn(context.Background(), tid, trn, 1)
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeDocsIndexNotReady {
		t.Fatalf("docs: %v", err)
	}
	d.docs.usable = true

	d.rate.err = apperr.RateLimited(0)
	d.rate.retryAfter = 0 // no remap
	_, err = d.service().RetryTurn(context.Background(), tid, trn, 1)
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeRateLimited {
		t.Fatalf("rate: %v", err)
	}
	d.rate.err = nil

	d.floor.assertErr = apperr.FloorLocked(1, 2)
	_, err = d.service().RetryTurn(context.Background(), tid, trn, 1)
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeFloorLocked {
		t.Fatalf("floor: %v", err)
	}
	d.floor.assertErr = nil

	d.locks.busy = true
	_, err = d.service().RetryTurn(context.Background(), tid, trn, 1)
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeThreadBusy {
		t.Fatalf("busy: %v", err)
	}
}

func TestInterruptTurn_ThreadMissing(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	tid, _ := valueobject.NewThreadID("thr_x")
	trn, _ := valueobject.NewTurnID("trn_1")
	err := d.service().InterruptTurn(context.Background(), tid, trn, 1)
	ae, ok := apperr.As(err)
	if !ok || ae.Code != apperr.CodeNotFound {
		t.Fatalf("got %v", err)
	}
}

func TestRetryTurn_ThreadMissing(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	tid, _ := valueobject.NewThreadID("thr_x")
	trn, _ := valueobject.NewTurnID("trn_1")
	_, err := d.service().RetryTurn(context.Background(), tid, trn, 1)
	ae, ok := apperr.As(err)
	if !ok || ae.Code != apperr.CodeNotFound {
		t.Fatalf("got %v", err)
	}
}

func TestCreateThread_HappyAndDocsGate(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	out, err := d.service().CreateThread(context.Background(), 11)
	if err != nil {
		t.Fatal(err)
	}
	if out.CreatedByAdminUserID != 11 || out.ThreadID.String() == "" {
		t.Fatalf("%+v", out)
	}
	d.docs.usable = false
	_, err = d.service().CreateThread(context.Background(), 11)
	ae, ok := apperr.As(err)
	if !ok || ae.Code != apperr.CodeDocsIndexNotReady {
		t.Fatalf("got %v", err)
	}
}

func TestGetThread_BusyAndFloor(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	tid, _ := valueobject.NewThreadID("thr_1")
	d.threads.threads[tid.String()] = &entity.ThreadSnapshot{ThreadID: tid, SeqHead: 9}
	d.locks.busy = true
	holder := int64(2)
	d.floor.remainingBy = map[string]struct {
		holder *int64
		sec    int
		err    error
	}{"thr_1": {holder: &holder, sec: 12}}
	snap, err := d.service().GetThread(context.Background(), tid, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !snap.Busy || snap.FloorRemainingSec != 12 || snap.FloorHolderAdminID == nil || *snap.FloorHolderAdminID != 2 {
		t.Fatalf("%+v", snap)
	}
}

func TestRenameThread_HappyAndMissing(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	tid, _ := valueobject.NewThreadID("thr_1")
	d.threads.threads[tid.String()] = &entity.ThreadSnapshot{ThreadID: tid}
	title, _ := valueobject.NewTitle("Renamed")
	out, err := d.service().RenameThread(context.Background(), tid, title)
	if err != nil || out.Title != title {
		t.Fatalf("%+v %v", out, err)
	}
	missing, _ := valueobject.NewThreadID("thr_x")
	_, err = d.service().RenameThread(context.Background(), missing, title)
	ae, ok := apperr.As(err)
	if !ok || ae.Code != apperr.CodeNotFound {
		t.Fatalf("got %v", err)
	}
}

func TestSubscribeEvents(t *testing.T) {
	t.Parallel()
	d := newMemDeps()
	tid, _ := valueobject.NewThreadID("thr_1")
	d.threads.threads[tid.String()] = &entity.ThreadSnapshot{ThreadID: tid}
	called := false
	err := d.service().SubscribeEvents(context.Background(), tid, 3, func(string, map[string]any) error {
		called = true
		return nil
	})
	if err != nil || !d.events.called || d.events.afterSeq != 3 {
		t.Fatalf("err=%v events=%+v called=%v", err, d.events, called)
	}
	missing, _ := valueobject.NewThreadID("thr_x")
	err = d.service().SubscribeEvents(context.Background(), missing, 0, nil)
	ae, ok := apperr.As(err)
	if !ok || ae.Code != apperr.CodeNotFound {
		t.Fatalf("got %v", err)
	}
}

// --- fakes ---

type identityRedactor struct{}

func (identityRedactor) Redact(_ context.Context, text string) (string, error) { return text, nil }

type errRedactor struct{ err error }

func (e errRedactor) Redact(context.Context, string) (string, error) { return "", e.err }

type memStore struct {
	threads    map[string]*entity.ThreadSnapshot
	list       []entity.ConversationMeta
	listErr    error
	createErr  error
	seqHeadErr error
	renameErr  error
}

func (m *memStore) CreateThread(_ context.Context, admin int64) (entity.ThreadSnapshot, error) {
	if m.createErr != nil {
		return entity.ThreadSnapshot{}, m.createErr
	}
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
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.list, nil
}
func (m *memStore) RenameThread(_ context.Context, id valueobject.ThreadID, title valueobject.Title) error {
	if m.renameErr != nil {
		return m.renameErr
	}
	_ = id
	_ = title
	return nil
}
func (m *memStore) SoftDeleteThread(context.Context, valueobject.ThreadID) error { return nil }
func (m *memStore) SeqHead(_ context.Context, id valueobject.ThreadID) (uint64, error) {
	if m.seqHeadErr != nil {
		return 0, m.seqHeadErr
	}
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

type memLock struct {
	busy     bool
	released bool
}

func (m *memLock) TryAcquire(context.Context, valueobject.ThreadID) (string, error) {
	if m.busy {
		return "", apperr.ThreadBusy()
	}
	return "tok", nil
}
func (m *memLock) Release(context.Context, valueobject.ThreadID, string) error {
	m.released = true
	return nil
}
func (m *memLock) IsBusy(context.Context, valueobject.ThreadID) (bool, error) { return m.busy, nil }

type memInterrupt struct {
	threadID valueobject.ThreadID
	turnID   valueobject.TurnID
}

func (m *memInterrupt) Request(_ context.Context, tid valueobject.ThreadID, trn valueobject.TurnID) error {
	m.threadID = tid
	m.turnID = trn
	return nil
}
func (memInterrupt) IsRequested(context.Context, valueobject.ThreadID, valueobject.TurnID) (bool, error) {
	return false, nil
}
func (memInterrupt) Clear(context.Context, valueobject.ThreadID, valueobject.TurnID) error {
	return nil
}

type memFloor struct {
	assertErr   error
	acquireErr  error
	remainingBy map[string]struct {
		holder *int64
		sec    int
		err    error
	}
}

func (m *memFloor) Assert(context.Context, valueobject.ThreadID, int64) error { return m.assertErr }
func (m *memFloor) Acquire(context.Context, valueobject.ThreadID, int64, valueobject.TurnID) error {
	return m.acquireErr
}
func (m *memFloor) Remaining(_ context.Context, id valueobject.ThreadID) (*int64, int, error) {
	if m.remainingBy == nil {
		return nil, 0, nil
	}
	r, ok := m.remainingBy[id.String()]
	if !ok {
		return nil, 0, nil
	}
	return r.holder, r.sec, r.err
}

type memRate struct {
	err        error
	retryAfter int
}

func (m *memRate) Assert(context.Context, int64) (int, error) {
	return m.retryAfter, m.err
}

type memDocs struct {
	usable bool
	err    error
}

func (m *memDocs) Gate(context.Context) (entity.DocsIndexGate, error) {
	if m.err != nil {
		return entity.DocsIndexGate{}, m.err
	}
	return entity.DocsIndexGate{Usable: m.usable, Status: "ready"}, nil
}
func (m *memDocs) Search(context.Context, string, int) (any, error) { return nil, nil }
func (m *memDocs) Reindex(context.Context) error                    { return nil }

type memWorker struct {
	called bool
	job    service.TurnJob
	err    error
}

func (m *memWorker) Enqueue(_ context.Context, job service.TurnJob) error {
	m.called = true
	m.job = job
	return m.err
}

type memEvents struct {
	called   bool
	afterSeq uint64
}

func (m *memEvents) Subscribe(_ context.Context, _ valueobject.ThreadID, afterSeq uint64, emit service.EventEmitFn) error {
	m.called = true
	m.afterSeq = afterSeq
	if emit != nil {
		_ = emit("item.completed", map[string]any{"ok": true})
	}
	return nil
}
