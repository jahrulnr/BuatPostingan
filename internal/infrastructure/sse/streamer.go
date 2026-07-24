// Package sse tails durable JSONL and emits SSE-shaped application events.
package sse

import (
	"context"
	"time"

	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/enum"
	"buatpostingan/internal/domain/repository"
	"buatpostingan/internal/domain/service"
	"buatpostingan/internal/domain/valueobject"
)

// Streamer implements service.EventStreamer via hub notify + slow safety poll.
type Streamer struct {
	store     repository.ThreadStore
	hub       *Hub
	pollEvery time.Duration
}

var _ service.EventStreamer = (*Streamer)(nil)

// NewStreamer creates an EventStreamer. hub may be nil (safety poll only).
func NewStreamer(store repository.ThreadStore, hub *Hub) *Streamer {
	return &Streamer{
		store: store,
		hub:   hub,
		// Safety net for missed wakes — primary path is hub.Notify on append.
		pollEvery: 1500 * time.Millisecond,
	}
}

// Subscribe emits durable JSONL events (with seq) and ephemeral hub deltas (no seq).
// Keepalive `: ping` is handled by the HTTP adapter (WriteSSEComment every 15s).
func (s *Streamer) Subscribe(ctx context.Context, threadID valueobject.ThreadID, afterSeq uint64, emit service.EventEmitFn) error {
	cursor := afterSeq
	ticker := time.NewTicker(s.pollEvery)
	defer ticker.Stop()

	var sub service.ThreadEventSub
	if s.hub != nil {
		sub = s.hub.Subscribe(threadID)
		defer sub.Close()
	}

	var notifyCh <-chan struct{}
	var ephemeralCh <-chan service.EphemeralEvent
	if sub != nil {
		notifyCh = sub.Notify()
		ephemeralCh = sub.Ephemeral()
	}

	for {
		if err := s.emitNew(ctx, threadID, &cursor, emit); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-notifyCh:
			// Durable append wake — loop emits new seqs.
		case ev, ok := <-ephemeralCh:
			if !ok {
				ephemeralCh = nil
				continue
			}
			payload := ev.Payload
			if payload == nil {
				payload = map[string]any{}
			}
			// Ephemeral frames must not carry durable seq (FE must not advance cursor).
			delete(payload, "seq")
			if err := emit(ev.Name, payload); err != nil {
				return err
			}
		case <-ticker.C:
		}
	}
}

func (s *Streamer) emitNew(ctx context.Context, threadID valueobject.ThreadID, cursor *uint64, emit service.EventEmitFn) error {
	snap, err := s.store.GetThread(ctx, threadID, *cursor)
	if err != nil {
		return err
	}
	for _, it := range snap.Items {
		if it.Seq <= *cursor {
			continue
		}
		ev, data := mapItem(it)
		if err := emit(ev, data); err != nil {
			return err
		}
		*cursor = it.Seq
	}
	return nil
}

func mapItem(it entity.TranscriptItem) (event string, data map[string]any) {
	line := itemToMap(it)
	switch it.Type {
	case enum.ItemTurnStarted:
		return "turn.started", line
	case enum.ItemTurnCompleted:
		return "turn.completed", line
	case enum.ItemTurnFailed:
		return "turn.failed", line
	case enum.ItemTurnResumed:
		return "turn.resumed", line
	case enum.ItemThreadStarted:
		return "thread.started", line
	case enum.ItemUserMessage, enum.ItemAgentMessage, enum.ItemToolCall, enum.ItemToolResult, enum.ItemReasoning:
		return "item.completed", wrapItem(line)
	default:
		return "item.updated", line
	}
}

func wrapItem(line map[string]any) map[string]any {
	return map[string]any{
		"type":      line["type"],
		"seq":       line["seq"],
		"thread_id": line["thread_id"],
		"turn_id":   line["turn_id"],
		"ts":        line["ts"],
		"item":      line,
	}
}

func itemToMap(it entity.TranscriptItem) map[string]any {
	m := map[string]any{
		"seq":       it.Seq,
		"thread_id": it.ThreadID.String(),
		"type":      string(it.Type),
		"id":        it.ID.String(),
		"ts":        float64(it.At.UnixNano()) / 1e9,
	}
	if it.TurnID.String() != "" {
		m["turn_id"] = it.TurnID.String()
	}
	for k, v := range it.Payload {
		m[k] = v
	}
	return m
}
