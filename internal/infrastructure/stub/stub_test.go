package stub_test

import (
	"context"
	"testing"

	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/service"
	"buatpostingan/internal/domain/valueobject"
	"buatpostingan/internal/infrastructure/stub"
	"buatpostingan/internal/pkg/apperr"
)

func TestStubPortsReturnNotImplemented(t *testing.T) {
	ctx := context.Background()
	tid, _ := valueobject.NewThreadID("thr_stub_zzzzzzzzzzzzzzzzz")
	turn, _ := valueobject.NewTurnID("trn_stub_zzzzzzzzzzzzzzzzz")
	title, _ := valueobject.NewTitle("t")
	itemID, _ := valueobject.NewItemID("itm_stub_zzzzzzzzzzzzzzzzz")

	mustNI := func(err error) {
		t.Helper()
		if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeNotImplemented {
			t.Fatalf("want not_implemented got %v", err)
		}
	}

	var store stub.ThreadStore
	_, err := store.CreateThread(ctx, 1)
	mustNI(err)
	_, err = store.GetThread(ctx, tid, 0)
	mustNI(err)
	_, err = store.AppendItem(ctx, tid, entity.TranscriptItem{ID: itemID})
	mustNI(err)
	_, err = store.ListConversations(ctx)
	mustNI(err)
	mustNI(store.RenameThread(ctx, tid, title))
	mustNI(store.SoftDeleteThread(ctx, tid))
	_, err = store.SeqHead(ctx, tid)
	mustNI(err)
	_, _, err = store.ResolveConversation(ctx, tid)
	mustNI(err)
	mustNI(store.AppendConversationMeta(ctx, entity.ConversationMeta{}))
	mustNI(store.ClearActiveTurn(ctx, tid))

	var lock stub.ThreadLock
	_, err = lock.TryAcquire(ctx, tid)
	mustNI(err)
	mustNI(lock.Release(ctx, tid, "x"))
	_, err = lock.IsBusy(ctx, tid)
	mustNI(err)

	var intr stub.InterruptFlag
	mustNI(intr.Request(ctx, tid, turn))
	_, err = intr.IsRequested(ctx, tid, turn)
	mustNI(err)
	mustNI(intr.Clear(ctx, tid, turn))

	var floor stub.SpeakFloor
	mustNI(floor.Assert(ctx, tid, 1))
	mustNI(floor.Acquire(ctx, tid, 1, turn))
	h, rem, err := floor.Remaining(ctx, tid)
	if err != nil || h != nil || rem != 0 {
		t.Fatalf("Remaining soft: %v %d %v", h, rem, err)
	}

	var rl stub.TurnRateLimit
	_, err = rl.Assert(ctx, 1)
	mustNI(err)

	var red stub.SecretRedactor
	out, err := red.Redact(ctx, "secret")
	if err != nil || out != "secret" {
		t.Fatalf("Redact identity: %q %v", out, err)
	}

	var docs stub.DocsIndex
	_, err = docs.Gate(ctx)
	mustNI(err)
	_, err = docs.Search(ctx, "q", 1)
	mustNI(err)
	mustNI(docs.Reindex(ctx))

	var sse stub.EventStreamer
	mustNI(sse.Subscribe(ctx, tid, 0, nil))

	var worker stub.TurnWorker
	mustNI(worker.Enqueue(ctx, service.TurnJob{}))
}
