package jsonl

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/enum"
	"buatpostingan/internal/domain/valueobject"
)

func TestRemainingSecTTLClamp(t *testing.T) {
	f := &SpeakFloor{ttlSec: 0}
	holder := int64(1)
	now := time.Now().UTC()
	meta := entity.ConversationMeta{
		FloorHolderAdminID: &holder,
		FloorLastTurnAt:    &now,
	}
	// ttlSec 0 → clamp to 1 inside remainingSec
	if rem := f.remainingSec(meta); rem < 1 {
		t.Fatalf("rem=%d", rem)
	}
	meta.FloorLastTurnAt = nil
	if f.remainingSec(meta) != 0 {
		t.Fatal("nil last turn")
	}
}

func TestMetaRowRoundTripFull(t *testing.T) {
	tid, _ := valueobject.NewThreadID("thr_metafull_zzzzzzzzzzzz")
	title, _ := valueobject.NewTitle("Hello")
	turn, _ := valueobject.NewTurnID("trn_metafull_zzzzzzzzzzzz")
	holder := int64(7)
	now := time.Now().UTC()
	meta := entity.ConversationMeta{
		ThreadID:                   tid,
		Title:                      &title,
		TitleSource:                enum.TitleManual,
		Status:                     enum.ConversationActive,
		CreatedByAdminUserID:       7,
		UpdatedAt:                  now,
		LastActivityAt:             now,
		FloorHolderAdminID:         &holder,
		FloorLastTurnAt:            &now,
		ActiveTurnID:               &turn,
		ActiveTurnInitiatorAdminID: &holder,
	}
	row := metaToRow(meta, "/tmp/x.jsonl")
	back, err := rowToMeta(row)
	if err != nil {
		t.Fatal(err)
	}
	if back.Title == nil || back.Title.String() != "Hello" {
		t.Fatalf("%+v", back)
	}
	if back.ActiveTurnID == nil || back.FloorLastTurnAt == nil {
		t.Fatalf("%+v", back)
	}

	// AdminUserID fallback when CreatedBy is 0
	row2 := sessionMetaRow{ThreadID: tid.String(), AdminUserID: 9}
	back2, err := rowToMeta(row2)
	if err != nil || back2.CreatedByAdminUserID != 9 {
		t.Fatalf("%+v %v", back2, err)
	}

	if timeToUnixFloat(time.Time{}) != 0 {
		t.Fatal("zero time")
	}
}

func TestWithFileLockCreatesAndDirOf(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "f.lock")
	err := withFileLock(path, func(f *os.File) error {
		_, err := f.Write([]byte("{}"))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if dirOf(path) == "" {
		t.Fatal("dirOf")
	}
}

func TestLineMapIncompleteAndBadTurn(t *testing.T) {
	_, err := lineMapToItem(map[string]any{"id": "x"})
	if err == nil {
		t.Fatal("incomplete")
	}
	item, err := lineMapToItem(map[string]any{
		"seq":       uint64(3),
		"ts":        float64(1),
		"thread_id": "thr_ok_zzzzzzzzzzzzzzzzzzz",
		"type":      "user_message",
		"id":        "itm_ok_zzzzzzzzzzzzzzzzzzz",
		"turn_id":   "", // empty ignored
	})
	if err != nil || item.Seq != 3 {
		t.Fatalf("%+v %v", item, err)
	}
}

func TestReadThreadLinesMissing(t *testing.T) {
	s := NewStore(t.TempDir())
	items, err := s.readThreadLines("thr_none")
	if err != nil || items != nil {
		t.Fatalf("%v %#v", err, items)
	}
}

func TestGetThreadSeqHeadFromSeqFile(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)
	ctx := context.Background()
	snap, err := s.CreateThread(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	// Bump seq file ahead of jsonl lines
	_ = os.WriteFile(s.threadSeqPath(snap.ThreadID.String()), []byte("99"), 0o644)
	got, err := s.GetThread(ctx, snap.ThreadID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.SeqHead != 99 {
		t.Fatalf("seq_head=%d", got.SeqHead)
	}
}

func TestNextSeqCorrupt(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)
	tid := "thr_corruptseq_zzzzzzzzzzz"
	path := s.threadSeqPath(tid)
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte("nope"), 0o644)
	_, err := s.nextSeq(tid)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestListConversationsActivityFallback(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)
	ctx := context.Background()
	tid1, _ := valueobject.NewThreadID("thr_list1_zzzzzzzzzzzzzzzz")
	tid2, _ := valueobject.NewThreadID("thr_list2_zzzzzzzzzzzzzzzz")
	old := time.Now().UTC().Add(-time.Hour)
	newer := time.Now().UTC()
	_ = s.AppendConversationMeta(ctx, entity.ConversationMeta{
		ThreadID: tid1, CreatedByAdminUserID: 1, LastActivityAt: old,
	})
	_ = s.AppendConversationMeta(ctx, entity.ConversationMeta{
		ThreadID: tid2, CreatedByAdminUserID: 2, LastActivityAt: newer,
	})
	list, err := s.ListConversations(ctx)
	if err != nil || len(list) < 2 {
		t.Fatalf("%v %#v", err, list)
	}
	if list[0].ThreadID != tid2 {
		t.Fatalf("want newer first, got %s", list[0].ThreadID)
	}
}

func TestWithFileLockMkdirFails(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "blocked")
	_ = os.WriteFile(parent, []byte("not-a-dir"), 0o644)
	err := withFileLock(filepath.Join(parent, "x.lock"), func(*os.File) error { return nil })
	if err == nil {
		t.Fatal("expected mkdir fail")
	}
}

func TestAppendJSONLLineMkdirFails(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "blocked")
	_ = os.WriteFile(parent, []byte("x"), 0o644)
	err := appendJSONLLine(filepath.Join(parent, "a.jsonl"), map[string]any{"a": 1})
	if err == nil {
		t.Fatal("expected fail")
	}
}

func TestMetaToRowEmptyDefaults(t *testing.T) {
	tid, _ := valueobject.NewThreadID("thr_emptyrow_zzzzzzzzzzzz")
	row := metaToRow(entity.ConversationMeta{ThreadID: tid}, "p")
	if row.TitleSource == "" || row.Status == "" {
		// metaToRow fills empties
		t.Fatalf("%+v", row)
	}
	// Clear to hit empty fill branches — pass empty strings explicitly via zero meta
	row = metaToRow(entity.ConversationMeta{ThreadID: tid, TitleSource: "", Status: ""}, "p")
	if row.TitleSource != string(enum.TitlePending) || row.Status != string(enum.ConversationActive) {
		t.Fatalf("%+v", row)
	}
}

