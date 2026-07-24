package sse

import (
	"context"
	"sync"
	"testing"
	"time"

	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/enum"
	"buatpostingan/internal/domain/valueobject"
)

func TestHubNotifyWakesSubscribe(t *testing.T) {
	hub := NewHub()
	tid, _ := valueobject.NewThreadID("thr_hub")
	store := &memStore{}
	s := NewStreamer(store, hub)
	s.pollEvery = 30 * time.Second // must wake via hub, not poll

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	var once sync.Once
	go func() {
		errCh <- s.Subscribe(ctx, tid, 0, func(ev string, data map[string]any) error {
			if ev != "item.completed" {
				t.Errorf("ev=%q", ev)
			}
			once.Do(cancel)
			return nil
		})
	}()

	// Give Subscribe time to register with the hub.
	time.Sleep(30 * time.Millisecond)
	store.mu.Lock()
	store.items = append(store.items, entity.TranscriptItem{
		Seq: 1, ThreadID: tid, Type: enum.ItemAgentMessage, ID: "itm_1",
		At: time.Now().UTC(), Payload: map[string]any{"text": "hi"},
	})
	store.mu.Unlock()
	hub.Notify(tid)

	select {
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			t.Fatalf("err=%v", err)
		}
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("hub notify did not wake Subscribe")
	}
}

func TestHubPublishEphemeral(t *testing.T) {
	hub := NewHub()
	tid, _ := valueobject.NewThreadID("thr_delta")
	store := &memStore{}
	s := NewStreamer(store, hub)
	s.pollEvery = 30 * time.Second

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	got := make(chan map[string]any, 1)
	go func() {
		errCh <- s.Subscribe(ctx, tid, 0, func(ev string, data map[string]any) error {
			if ev == "item.delta" {
				got <- data
				cancel()
			}
			return nil
		})
	}()

	time.Sleep(30 * time.Millisecond)
	hub.PublishEphemeral(tid, "item.delta", map[string]any{
		"type":    "agent_message",
		"turn_id": "trn_1",
		"delta":   "Hel",
		"seq":     uint64(99), // must be stripped
	})

	select {
	case data := <-got:
		if data["delta"] != "Hel" {
			t.Fatalf("data=%#v", data)
		}
		if _, ok := data["seq"]; ok {
			t.Fatalf("ephemeral must not carry seq: %#v", data)
		}
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("no ephemeral event")
	}
	<-errCh
}

func TestNotifyingStoreAppendNotifies(t *testing.T) {
	hub := NewHub()
	tid, _ := valueobject.NewThreadID("thr_n")
	inner := &memAppendStore{tid: tid}
	n := &NotifyingStore{Inner: inner, Hub: hub}
	sub := hub.Subscribe(tid)
	defer sub.Close()

	_, err := n.AppendItem(context.Background(), tid, entity.TranscriptItem{
		ID: "itm_x", Type: enum.ItemUserMessage, At: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-sub.Notify():
	case <-time.After(time.Second):
		t.Fatal("expected notify")
	}
}

type memAppendStore struct {
	memStore
	tid valueobject.ThreadID
	seq uint64
}

func (m *memAppendStore) AppendItem(_ context.Context, threadID valueobject.ThreadID, item entity.TranscriptItem) (entity.TranscriptItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seq++
	item.Seq = m.seq
	item.ThreadID = threadID
	m.items = append(m.items, item)
	return item, nil
}
