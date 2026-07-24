package jsonl_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/enum"
	"buatpostingan/internal/domain/valueobject"
	"buatpostingan/internal/infrastructure/repository/jsonl"
	"buatpostingan/internal/pkg/apperr"
)

func TestStoreCreateAppendList(t *testing.T) {
	root := t.TempDir()
	store := jsonl.NewStore(root)
	ctx := context.Background()

	snap, err := store.CreateThread(ctx, 42)
	if err != nil {
		t.Fatal(err)
	}
	if snap.SeqHead != 1 {
		t.Fatalf("seq_head=%d want 1", snap.SeqHead)
	}
	if snap.ThreadID.String() == "" {
		t.Fatal("empty thread id")
	}

	itemID, _ := valueobject.NewItemID("itm_msg1")
	turnID, _ := valueobject.NewTurnID("trn_1")
	got, err := store.AppendItem(ctx, snap.ThreadID, entity.TranscriptItem{
		ID:       itemID,
		ThreadID: snap.ThreadID,
		TurnID:   turnID,
		Type:     enum.ItemUserMessage,
		Payload: map[string]any{
			"text":          "hello",
			"admin_user_id": int64(42),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Seq != 2 {
		t.Fatalf("seq=%d want 2", got.Seq)
	}

	hydrated, err := store.GetThread(ctx, snap.ThreadID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if hydrated.SeqHead != 2 {
		t.Fatalf("seq_head=%d want 2", hydrated.SeqHead)
	}
	if len(hydrated.Items) != 2 {
		t.Fatalf("items=%d want 2", len(hydrated.Items))
	}
	text, _ := hydrated.Items[1].Payload["text"].(string)
	if text != "hello" {
		t.Fatalf("text=%q", text)
	}
	adminID, _ := hydrated.Items[1].Payload["admin_user_id"].(int64)
	if adminID != 42 {
		t.Fatalf("admin_user_id=%v", hydrated.Items[1].Payload["admin_user_id"])
	}

	after, err := store.GetThread(ctx, snap.ThreadID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Items) != 1 || after.Items[0].Seq != 2 {
		t.Fatalf("after_seq filter failed: %+v", after.Items)
	}

	list, err := store.ListConversations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].CreatedByAdminUserID != 42 {
		t.Fatalf("list=%+v", list)
	}

	title, _ := valueobject.NewTitle("Renamed")
	if err := store.RenameThread(ctx, snap.ThreadID, title); err != nil {
		t.Fatal(err)
	}
	meta, ok, err := store.ResolveConversation(ctx, snap.ThreadID)
	if err != nil || !ok {
		t.Fatalf("resolve: ok=%v err=%v", ok, err)
	}
	if meta.Title == nil || meta.Title.String() != "Renamed" || meta.TitleSource != enum.TitleManual {
		t.Fatalf("rename meta=%+v", meta)
	}

	if _, err := os.Stat(filepath.Join(root, "session_index.jsonl")); err != nil {
		t.Fatal(err)
	}
}

func TestLockBusyHardened(t *testing.T) {
	root := t.TempDir()
	lock := jsonl.NewLock(root, 300)
	ctx := context.Background()
	tid, _ := valueobject.NewThreadID("thr_lock")

	tok, err := lock.TryAcquire(ctx, tid)
	if err != nil {
		t.Fatal(err)
	}
	if tok == "" {
		t.Fatal("empty token")
	}
	busy, err := lock.IsBusy(ctx, tid)
	if err != nil || !busy {
		t.Fatalf("busy=%v err=%v", busy, err)
	}
	_, err = lock.TryAcquire(ctx, tid)
	if err == nil {
		t.Fatal("expected busy on second acquire")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeThreadBusy {
		t.Fatalf("want thread_busy got %v", err)
	}
	if err := lock.Release(ctx, tid, "wrong"); err != nil {
		t.Fatal(err)
	}
	still, _ := lock.IsBusy(ctx, tid)
	if !still {
		t.Fatal("wrong token should not release")
	}
	if err := lock.Release(ctx, tid, tok); err != nil {
		t.Fatal(err)
	}
	busy, _ = lock.IsBusy(ctx, tid)
	if busy {
		t.Fatal("expected free after release")
	}
}

func TestInterruptFlag(t *testing.T) {
	root := t.TempDir()
	intr := jsonl.NewInterrupt(root)
	ctx := context.Background()
	tid, _ := valueobject.NewThreadID("thr_i")
	turn, _ := valueobject.NewTurnID("trn_i")

	ok, err := intr.IsRequested(ctx, tid, turn)
	if err != nil || ok {
		t.Fatalf("before: ok=%v err=%v", ok, err)
	}
	if err := intr.Request(ctx, tid, turn); err != nil {
		t.Fatal(err)
	}
	ok, err = intr.IsRequested(ctx, tid, turn)
	if err != nil || !ok {
		t.Fatalf("after: ok=%v err=%v", ok, err)
	}
	if err := intr.Clear(ctx, tid, turn); err != nil {
		t.Fatal(err)
	}
	ok, _ = intr.IsRequested(ctx, tid, turn)
	if ok {
		t.Fatal("expected cleared")
	}
}

func TestSpeakFloorAssertAcquire(t *testing.T) {
	root := t.TempDir()
	store := jsonl.NewStore(root)
	floor := jsonl.NewSpeakFloor(store, 600)
	ctx := context.Background()

	snap, err := store.CreateThread(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	turn, _ := valueobject.NewTurnID("trn_floor")
	if err := floor.Acquire(ctx, snap.ThreadID, 1, turn); err != nil {
		t.Fatal(err)
	}
	if err := floor.Assert(ctx, snap.ThreadID, 1); err != nil {
		t.Fatal(err)
	}
	err = floor.Assert(ctx, snap.ThreadID, 2)
	if err == nil {
		t.Fatal("expected floor locked for other admin")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeFloorLocked {
		t.Fatalf("want floor_locked got %v", err)
	}
	holder, rem, err := floor.Remaining(ctx, snap.ThreadID)
	if err != nil || holder == nil || *holder != 1 || rem <= 0 {
		t.Fatalf("holder=%v rem=%d err=%v", holder, rem, err)
	}
}
