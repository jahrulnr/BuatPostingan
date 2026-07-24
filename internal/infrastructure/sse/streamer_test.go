package sse

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/enum"
	"buatpostingan/internal/domain/valueobject"
)

type memStore struct {
	mu    sync.Mutex
	items []entity.TranscriptItem
	err   error
}

func (m *memStore) GetThread(_ context.Context, _ valueobject.ThreadID, afterSeq uint64) (entity.ThreadSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return entity.ThreadSnapshot{}, m.err
	}
	var out []entity.TranscriptItem
	for _, it := range m.items {
		if it.Seq > afterSeq {
			out = append(out, it)
		}
	}
	return entity.ThreadSnapshot{Items: out}, nil
}

func (m *memStore) CreateThread(context.Context, int64) (entity.ThreadSnapshot, error) {
	return entity.ThreadSnapshot{}, errors.New("unused")
}
func (m *memStore) AppendItem(context.Context, valueobject.ThreadID, entity.TranscriptItem) (entity.TranscriptItem, error) {
	return entity.TranscriptItem{}, errors.New("unused")
}
func (m *memStore) ListConversations(context.Context) ([]entity.ConversationMeta, error) {
	return nil, errors.New("unused")
}
func (m *memStore) RenameThread(context.Context, valueobject.ThreadID, valueobject.Title) error {
	return errors.New("unused")
}
func (m *memStore) SoftDeleteThread(context.Context, valueobject.ThreadID) error {
	return errors.New("unused")
}
func (m *memStore) SeqHead(context.Context, valueobject.ThreadID) (uint64, error) {
	return 0, errors.New("unused")
}
func (m *memStore) ResolveConversation(context.Context, valueobject.ThreadID) (entity.ConversationMeta, bool, error) {
	return entity.ConversationMeta{}, false, errors.New("unused")
}
func (m *memStore) AppendConversationMeta(context.Context, entity.ConversationMeta) error {
	return errors.New("unused")
}
func (m *memStore) ClearActiveTurn(context.Context, valueobject.ThreadID) error {
	return errors.New("unused")
}

func TestNewStreamer(t *testing.T) {
	s := NewStreamer(&memStore{})
	if s == nil || s.pollEvery != 500*time.Millisecond {
		t.Fatalf("got %#v", s)
	}
}

func TestMapItem(t *testing.T) {
	tid, _ := valueobject.NewThreadID("thr_1")
	turn, _ := valueobject.NewTurnID("trn_1")
	iid, _ := valueobject.NewItemID("itm_1")
	base := entity.TranscriptItem{
		Seq: 1, ID: iid, ThreadID: tid, TurnID: turn,
		Type: enum.ItemTurnStarted, At: time.Unix(1, 0).UTC(),
		Payload: map[string]any{"k": "v"},
	}
	tests := []struct {
		typ   enum.ItemType
		event string
		wrap  bool
	}{
		{enum.ItemTurnStarted, "turn.started", false},
		{enum.ItemTurnCompleted, "turn.completed", false},
		{enum.ItemTurnFailed, "turn.failed", false},
		{enum.ItemTurnResumed, "turn.resumed", false},
		{enum.ItemThreadStarted, "thread.started", false},
		{enum.ItemUserMessage, "item.completed", true},
		{enum.ItemAgentMessage, "item.completed", true},
		{enum.ItemToolCall, "item.completed", true},
		{enum.ItemToolResult, "item.completed", true},
		{enum.ItemReasoning, "item.completed", true},
		{enum.ItemType("custom"), "item.updated", false},
	}
	for _, tc := range tests {
		it := base
		it.Type = tc.typ
		ev, data := mapItem(it)
		if ev != tc.event {
			t.Fatalf("%s: event=%q want %q", tc.typ, ev, tc.event)
		}
		if tc.wrap {
			if _, ok := data["item"]; !ok {
				t.Fatalf("%s: want wrapped item, got %#v", tc.typ, data)
			}
		} else if data["type"] != string(tc.typ) {
			t.Fatalf("%s: type=%v", tc.typ, data["type"])
		}
		if data["k"] != "v" && !tc.wrap {
			t.Fatalf("%s: payload not merged: %#v", tc.typ, data)
		}
	}
}

func TestItemToMapOmitsEmptyTurn(t *testing.T) {
	tid, _ := valueobject.NewThreadID("thr_1")
	iid, _ := valueobject.NewItemID("itm_1")
	it := entity.TranscriptItem{
		Seq: 2, ID: iid, ThreadID: tid, Type: enum.ItemThreadStarted,
		At: time.Unix(2, 0).UTC(),
	}
	m := itemToMap(it)
	if _, ok := m["turn_id"]; ok {
		t.Fatalf("turn_id should be omitted: %#v", m)
	}
	if m["seq"] != uint64(2) || m["thread_id"] != "thr_1" {
		t.Fatalf("%#v", m)
	}
}

func TestEmitNewAdvancesCursorAndSkipsOld(t *testing.T) {
	tid, _ := valueobject.NewThreadID("thr_1")
	turn, _ := valueobject.NewTurnID("trn_1")
	store := &memStore{items: []entity.TranscriptItem{
		{Seq: 1, ThreadID: tid, TurnID: turn, Type: enum.ItemUserMessage, ID: "itm_1", At: time.Now().UTC(), Payload: map[string]any{"text": "a"}},
		{Seq: 2, ThreadID: tid, TurnID: turn, Type: enum.ItemAgentMessage, ID: "itm_2", At: time.Now().UTC(), Payload: map[string]any{"text": "b"}},
		{Seq: 3, ThreadID: tid, TurnID: turn, Type: enum.ItemTurnCompleted, ID: "itm_3", At: time.Now().UTC()},
	}}
	s := NewStreamer(store)
	var got []string
	cursor := uint64(1)
	if err := s.emitNew(context.Background(), tid, &cursor, func(ev string, _ map[string]any) error {
		got = append(got, ev)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if cursor != 3 {
		t.Fatalf("cursor=%d", cursor)
	}
	if len(got) != 2 || got[0] != "item.completed" || got[1] != "turn.completed" {
		t.Fatalf("got=%v", got)
	}
}

func TestEmitNewStoreError(t *testing.T) {
	s := NewStreamer(&memStore{err: errors.New("boom")})
	cursor := uint64(0)
	err := s.emitNew(context.Background(), "thr_x", &cursor, func(string, map[string]any) error { return nil })
	if err == nil || err.Error() != "boom" {
		t.Fatalf("err=%v", err)
	}
}

func TestEmitNewEmitError(t *testing.T) {
	tid, _ := valueobject.NewThreadID("thr_1")
	store := &memStore{items: []entity.TranscriptItem{
		{Seq: 1, ThreadID: tid, Type: enum.ItemTurnStarted, ID: "itm_1", At: time.Now().UTC()},
	}}
	s := NewStreamer(store)
	cursor := uint64(0)
	want := errors.New("emit fail")
	err := s.emitNew(context.Background(), tid, &cursor, func(string, map[string]any) error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("err=%v", err)
	}
	if cursor != 0 {
		t.Fatalf("cursor should not advance on emit error, got %d", cursor)
	}
}

func TestSubscribeCancelsAfterEmit(t *testing.T) {
	tid, _ := valueobject.NewThreadID("thr_1")
	store := &memStore{items: []entity.TranscriptItem{
		{Seq: 5, ThreadID: tid, Type: enum.ItemTurnStarted, ID: "itm_1", At: time.Now().UTC()},
	}}
	s := NewStreamer(store)
	s.pollEvery = 20 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	var once sync.Once
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Subscribe(ctx, tid, 0, func(ev string, data map[string]any) error {
			if ev != "turn.started" {
				t.Errorf("ev=%q", ev)
			}
			if data["seq"] != uint64(5) {
				t.Errorf("data=%#v", data)
			}
			once.Do(cancel)
			return nil
		})
	}()
	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err=%v", err)
		}
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("Subscribe did not return")
	}
}

func TestSubscribeHitsTickerThenCancel(t *testing.T) {
	tid, _ := valueobject.NewThreadID("thr_2")
	store := &memStore{}
	s := NewStreamer(store)
	s.pollEvery = 15 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	err := s.Subscribe(ctx, tid, 0, func(string, map[string]any) error { return nil })
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err=%v", err)
	}
}
