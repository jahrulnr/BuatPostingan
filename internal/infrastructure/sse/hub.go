package sse

import (
	"sync"

	"buatpostingan/internal/domain/service"
	"buatpostingan/internal/domain/valueobject"
)

// Hub is an in-process fan-out for durable wakeups and ephemeral SSE deltas.
// Single-process only; multi-instance would need a shared bus later.
type Hub struct {
	mu   sync.RWMutex
	subs map[string]map[*hubSub]struct{}
}

var _ service.ThreadEventHub = (*Hub)(nil)

type hubSub struct {
	notify    chan struct{}
	ephemeral chan service.EphemeralEvent
}

// NewHub creates an empty thread event hub.
func NewHub() *Hub {
	return &Hub{subs: make(map[string]map[*hubSub]struct{})}
}

// Subscribe registers for durable wakes and ephemeral events on a thread.
func (h *Hub) Subscribe(threadID valueobject.ThreadID) service.ThreadEventSub {
	if h == nil {
		return noopSub{}
	}
	key := threadID.String()
	sub := &hubSub{
		notify:    make(chan struct{}, 1),
		ephemeral: make(chan service.EphemeralEvent, 64),
	}
	h.mu.Lock()
	if h.subs[key] == nil {
		h.subs[key] = make(map[*hubSub]struct{})
	}
	h.subs[key][sub] = struct{}{}
	h.mu.Unlock()
	return &hubSubscription{hub: h, key: key, sub: sub}
}

// Notify wakes durable subscribers (coalesced; buffer size 1).
func (h *Hub) Notify(threadID valueobject.ThreadID) {
	if h == nil {
		return
	}
	key := threadID.String()
	h.mu.RLock()
	defer h.mu.RUnlock()
	for sub := range h.subs[key] {
		select {
		case sub.notify <- struct{}{}:
		default:
		}
	}
}

// PublishEphemeral fans out a non-durable SSE event. Slow subscribers drop.
func (h *Hub) PublishEphemeral(threadID valueobject.ThreadID, eventName string, payload map[string]any) {
	if h == nil || eventName == "" {
		return
	}
	ev := service.EphemeralEvent{Name: eventName, Payload: payload}
	key := threadID.String()
	h.mu.RLock()
	defer h.mu.RUnlock()
	for sub := range h.subs[key] {
		select {
		case sub.ephemeral <- ev:
		default:
			// Drop when the client is slow — durable item.completed remains SoT.
		}
	}
}

type hubSubscription struct {
	hub *Hub
	key string
	sub *hubSub
}

func (s *hubSubscription) Notify() <-chan struct{} {
	if s == nil || s.sub == nil {
		return nil
	}
	return s.sub.notify
}

func (s *hubSubscription) Ephemeral() <-chan service.EphemeralEvent {
	if s == nil || s.sub == nil {
		return nil
	}
	return s.sub.ephemeral
}

func (s *hubSubscription) Close() {
	if s == nil || s.hub == nil || s.sub == nil {
		return
	}
	s.hub.mu.Lock()
	if room := s.hub.subs[s.key]; room != nil {
		delete(room, s.sub)
		if len(room) == 0 {
			delete(s.hub.subs, s.key)
		}
	}
	s.hub.mu.Unlock()
}

type noopSub struct{}

func (noopSub) Notify() <-chan struct{}                  { return nil }
func (noopSub) Ephemeral() <-chan service.EphemeralEvent { return nil }
func (noopSub) Close()                                   {}
