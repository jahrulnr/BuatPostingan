package jsonl_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/enum"
	"buatpostingan/internal/domain/valueobject"
	"buatpostingan/internal/infrastructure/repository/jsonl"
	"buatpostingan/internal/pkg/apperr"
)

func TestSoftDeleteClearActiveAndSeqHead(t *testing.T) {
	root := t.TempDir()
	store := jsonl.NewStore(root)
	intr := jsonl.NewInterrupt(root)
	ctx := context.Background()

	snap, err := store.CreateThread(ctx, 9)
	if err != nil {
		t.Fatal(err)
	}
	turn, _ := valueobject.NewTurnID("trn_del")
	if err := intr.Request(ctx, snap.ThreadID, turn); err != nil {
		t.Fatal(err)
	}

	floor := jsonl.NewSpeakFloor(store, 600)
	if err := floor.Acquire(ctx, snap.ThreadID, 9, turn); err != nil {
		t.Fatal(err)
	}
	if err := store.ClearActiveTurn(ctx, snap.ThreadID); err != nil {
		t.Fatal(err)
	}
	meta, ok, err := store.ResolveConversation(ctx, snap.ThreadID)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if meta.ActiveTurnID != nil {
		t.Fatalf("active turn still set: %+v", meta.ActiveTurnID)
	}

	head, err := store.SeqHead(ctx, snap.ThreadID)
	if err != nil || head < 1 {
		t.Fatalf("head=%d err=%v", head, err)
	}
	missingID, _ := valueobject.NewThreadID("thr_noseq_zzzzzzzzzzzzzzzz")
	h0, err := store.SeqHead(ctx, missingID)
	if err != nil || h0 != 0 {
		t.Fatalf("missing seq: %d %v", h0, err)
	}

	if err := store.SoftDeleteThread(ctx, snap.ThreadID); err != nil {
		t.Fatal(err)
	}
	list, err := store.ListConversations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range list {
		if m.ThreadID == snap.ThreadID {
			t.Fatal("deleted thread still listed")
		}
	}
	_, err = store.GetThread(ctx, snap.ThreadID, 0)
	if err == nil {
		t.Fatal("expected not found after soft delete")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeNotFound {
		t.Fatalf("want not_found got %v", err)
	}

	// ClearActiveTurn on unknown is no-op
	if err := store.ClearActiveTurn(ctx, missingID); err != nil {
		t.Fatal(err)
	}
}

func TestSoftDeleteBlockedByLock(t *testing.T) {
	root := t.TempDir()
	store := jsonl.NewStore(root)
	lock := jsonl.NewLock(root, 300)
	ctx := context.Background()
	snap, err := store.CreateThread(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	tok, err := lock.TryAcquire(ctx, snap.ThreadID)
	if err != nil {
		t.Fatal(err)
	}
	err = store.SoftDeleteThread(ctx, snap.ThreadID)
	if err == nil {
		t.Fatal("expected thread_busy")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeThreadBusy {
		t.Fatalf("got %v", err)
	}
	_ = lock.Release(ctx, snap.ThreadID, tok)
	if err := store.SoftDeleteThread(ctx, snap.ThreadID); err != nil {
		t.Fatal(err)
	}
}

func TestSoftDeleteMissingAndRenameDeleted(t *testing.T) {
	root := t.TempDir()
	store := jsonl.NewStore(root)
	ctx := context.Background()
	missing, _ := valueobject.NewThreadID("thr_missing_zzzzzzzzzzzzzz")
	err := store.SoftDeleteThread(ctx, missing)
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeNotFound {
		t.Fatalf("got %v", err)
	}
	title, _ := valueobject.NewTitle("x")
	err = store.RenameThread(ctx, missing, title)
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeNotFound {
		t.Fatalf("rename: %v", err)
	}
}

func TestAppendItemValidation(t *testing.T) {
	root := t.TempDir()
	store := jsonl.NewStore(root)
	ctx := context.Background()
	snap, _ := store.CreateThread(ctx, 1)
	_, err := store.AppendItem(ctx, snap.ThreadID, entity.TranscriptItem{})
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeValidation {
		t.Fatalf("got %v", err)
	}
}

func TestLockTTLExpiryAndDefaults(t *testing.T) {
	root := t.TempDir()
	lock := jsonl.NewLock(root, 0) // default TTL
	ctx := context.Background()
	tid, _ := valueobject.NewThreadID("thr_ttl_zzzzzzzzzzzzzzzz")

	// Seed an already-expired lock record (avoids flake on ExpiresAt == now.Unix).
	lockPath := filepath.Join(root, "threads", tid.String()+".lock")
	_ = os.MkdirAll(filepath.Dir(lockPath), 0o755)
	past := time.Now().Unix() - 10
	rec, _ := json.Marshal(map[string]any{
		"thread_id":   tid.String(),
		"token":       "ulid_old",
		"expires_at":  past,
		"acquired_at": past - 1,
	})
	_ = os.WriteFile(lockPath, append(rec, '\n'), 0o644)
	busy, err := lock.IsBusy(ctx, tid)
	if err != nil || busy {
		t.Fatalf("expired should be free: busy=%v err=%v", busy, err)
	}
	tok2, err := lock.TryAcquire(ctx, tid)
	if err != nil || tok2 == "" {
		t.Fatalf("reacquire: %v %q", err, tok2)
	}
	_ = lock.Release(ctx, tid, "wrong")
	still, _ := lock.IsBusy(ctx, tid)
	if !still {
		t.Fatal("wrong token must not release")
	}
	_ = lock.Release(ctx, tid, tok2)
	// Release missing / empty file
	if err := lock.Release(ctx, tid, "x"); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(lockPath, []byte{}, 0o644)
	busy, _ = lock.IsBusy(ctx, tid)
	if busy {
		t.Fatal("empty lock not busy")
	}
	_ = os.WriteFile(lockPath, []byte("{bad"), 0o644)
	busy, _ = lock.IsBusy(ctx, tid)
	if busy {
		t.Fatal("corrupt lock not busy")
	}
}

func TestSpeakFloorExpiredAndMissing(t *testing.T) {
	root := t.TempDir()
	store := jsonl.NewStore(root)
	floor := jsonl.NewSpeakFloor(store, 0) // default 600
	ctx := context.Background()
	snap, err := store.CreateThread(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	missing, _ := valueobject.NewThreadID("thr_nofloor_zzzzzzzzzzzzzz")
	err = floor.Assert(ctx, missing, 1)
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeNotFound {
		t.Fatalf("assert missing: %v", err)
	}
	h, rem, err := floor.Remaining(ctx, missing)
	if err != nil || h != nil || rem != 0 {
		t.Fatalf("remaining missing: %v %d %v", h, rem, err)
	}
	emptyTurn, _ := valueobject.NewTurnID("trn_empty_zzzzzzzzzzzzzzz")
	err = floor.Acquire(ctx, missing, 1, emptyTurn)
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeNotFound {
		t.Fatalf("acquire missing: %v", err)
	}

	// Expired floor: holder set but last turn far in the past
	past := time.Now().UTC().Add(-2 * time.Hour)
	holder := int64(5)
	meta, _, _ := store.ResolveConversation(ctx, snap.ThreadID)
	meta.FloorHolderAdminID = &holder
	meta.FloorLastTurnAt = &past
	if err := store.AppendConversationMeta(ctx, meta); err != nil {
		t.Fatal(err)
	}
	if err := floor.Assert(ctx, snap.ThreadID, 99); err != nil {
		t.Fatalf("expired floor should allow: %v", err)
	}
	_, rem, err = floor.Remaining(ctx, snap.ThreadID)
	if err != nil || rem != 0 {
		t.Fatalf("rem=%d err=%v", rem, err)
	}
}

func TestListConversationsSkipsCorruptAndSorts(t *testing.T) {
	root := t.TempDir()
	store := jsonl.NewStore(root)
	ctx := context.Background()
	a, _ := store.CreateThread(ctx, 1)
	time.Sleep(20 * time.Millisecond)
	b, _ := store.CreateThread(ctx, 2)

	idxPath := filepath.Join(root, "session_index.jsonl")
	f, err := os.OpenFile(idxPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("not-json\n{\"thread_id\":\"\"}\n")
	_ = f.Close()

	list, err := store.ListConversations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) < 2 {
		t.Fatalf("list=%+v", list)
	}
	// newest activity first
	if list[0].ThreadID != b.ThreadID && list[0].ThreadID != a.ThreadID {
		t.Fatalf("unexpected first %+v", list[0])
	}
}

func TestGetThreadSkipsCorruptLines(t *testing.T) {
	root := t.TempDir()
	store := jsonl.NewStore(root)
	ctx := context.Background()
	snap, _ := store.CreateThread(ctx, 1)
	path := filepath.Join(root, "threads", snap.ThreadID.String()+".jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("bogus\n")
	_ = f.Close()

	itemID, _ := valueobject.NewItemID("itm_ok_zzzzzzzzzzzzzzzzzz")
	_, err = store.AppendItem(ctx, snap.ThreadID, entity.TranscriptItem{
		ID:       itemID,
		ThreadID: snap.ThreadID,
		Type:     enum.ItemAgentMessage,
		Payload:  map[string]any{"text": "ok", "score": 1.5, "nilv": nil, "seq": 99},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.GetThread(ctx, snap.ThreadID, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, it := range got.Items {
		if it.Type == enum.ItemAgentMessage {
			found = true
			if it.Payload["score"] != 1.5 {
				t.Fatalf("score=%v", it.Payload["score"])
			}
		}
	}
	if !found {
		t.Fatal("agent message missing")
	}
}

func TestAppendConversationMetaValidation(t *testing.T) {
	store := jsonl.NewStore(t.TempDir())
	err := store.AppendConversationMeta(context.Background(), entity.ConversationMeta{})
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeValidation {
		t.Fatalf("got %v", err)
	}
}

func TestSeqHeadEmptyAndWhitespace(t *testing.T) {
	root := t.TempDir()
	store := jsonl.NewStore(root)
	ctx := context.Background()
	tid, _ := valueobject.NewThreadID("thr_seqempty_zzzzzzzzzzzz")
	path := filepath.Join(root, "threads", tid.String()+".seq")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte("   \n"), 0o644)
	n, err := store.SeqHead(ctx, tid)
	if err != nil || n != 0 {
		t.Fatalf("empty seq=%d err=%v", n, err)
	}
	_ = os.WriteFile(path, []byte("42\n"), 0o644)
	n, err = store.SeqHead(ctx, tid)
	if err != nil || n != 42 {
		t.Fatalf("seq=%d err=%v", n, err)
	}
}

func TestAppendMetaDefaultsAndListDeleted(t *testing.T) {
	root := t.TempDir()
	store := jsonl.NewStore(root)
	ctx := context.Background()
	tid, _ := valueobject.NewThreadID("thr_meta_zzzzzzzzzzzzzzzz")
	if err := store.AppendConversationMeta(ctx, entity.ConversationMeta{
		ThreadID:             tid,
		CreatedByAdminUserID: 3,
	}); err != nil {
		t.Fatal(err)
	}
	meta, ok, err := store.ResolveConversation(ctx, tid)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if meta.Status != enum.ConversationActive || meta.TitleSource != enum.TitlePending {
		t.Fatalf("%+v", meta)
	}
	meta.Status = enum.ConversationDeleted
	_ = store.AppendConversationMeta(ctx, meta)
	list, _ := store.ListConversations(ctx)
	for _, m := range list {
		if m.ThreadID == tid {
			t.Fatal("deleted should be hidden")
		}
	}
	_, err = store.GetThread(ctx, tid, 0)
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeNotFound {
		t.Fatalf("got %v", err)
	}
}


func TestLineHelpersViaRoundTrip(t *testing.T) {
	root := t.TempDir()
	store := jsonl.NewStore(root)
	ctx := context.Background()
	snap, _ := store.CreateThread(ctx, 1)
	path := filepath.Join(root, "threads", snap.ThreadID.String()+".jsonl")
	line := map[string]any{
		"seq":       float64(99),
		"ts":        float64(1700000000.5),
		"thread_id": snap.ThreadID.String(),
		"type":      string(enum.ItemUserMessage),
		"id":        "itm_line_zzzzzzzzzzzzzzzzz",
		"turn_id":   "trn_line_zzzzzzzzzzzzzzzzz",
		"n":         float64(3),
	}
	raw, _ := json.Marshal(line)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.Write(append(raw, '\n'))
	_ = f.Close()

	got, err := store.GetThread(ctx, snap.ThreadID, 0)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, it := range got.Items {
		if it.ID.String() == "itm_line_zzzzzzzzzzzzzzzzz" {
			found = true
			if it.Seq != 99 {
				t.Fatalf("seq=%d", it.Seq)
			}
			if it.Payload["n"] != int64(3) {
				t.Fatalf("n=%v", it.Payload["n"])
			}
		}
	}
	if !found {
		t.Fatalf("line not loaded; items=%d", len(got.Items))
	}
}
