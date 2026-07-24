package jsonl

import (
	"context"
	"math"
	"time"

	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/enum"
	"buatpostingan/internal/domain/repository"
	"buatpostingan/internal/domain/service"
	"buatpostingan/internal/domain/valueobject"
	"buatpostingan/internal/pkg/apperr"
)

const defaultSpeakFloorTTLSec = 600

// SpeakFloor enforces the speak-floor lock via session_index meta.
type SpeakFloor struct {
	store  repository.ThreadStore
	ttlSec int
}

var _ service.SpeakFloor = (*SpeakFloor)(nil)

// NewSpeakFloor wraps a ThreadStore that supports Resolve/Append meta
// (typically *jsonl.Store). ttlSec defaults to 600 when <= 0.
func NewSpeakFloor(store repository.ThreadStore, ttlSec int) *SpeakFloor {
	if ttlSec <= 0 {
		ttlSec = defaultSpeakFloorTTLSec
	}
	return &SpeakFloor{store: store, ttlSec: ttlSec}
}

func (f *SpeakFloor) Assert(ctx context.Context, threadID valueobject.ThreadID, adminUserID int64) error {
	meta, ok, err := f.store.ResolveConversation(ctx, threadID)
	if err != nil {
		return err
	}
	if !ok || meta.Status == enum.ConversationDeleted {
		return apperr.NotFound("thread not found")
	}
	holder := meta.FloorHolderAdminID
	if holder == nil || *holder == adminUserID {
		return nil
	}
	remaining := f.remainingSec(meta)
	if remaining == 0 {
		return nil
	}
	return apperr.FloorLocked(*holder, remaining)
}

func (f *SpeakFloor) Acquire(ctx context.Context, threadID valueobject.ThreadID, adminUserID int64, turnID valueobject.TurnID) error {
	prev, ok, err := f.store.ResolveConversation(ctx, threadID)
	if err != nil {
		return err
	}
	if !ok {
		return apperr.NotFound("thread not found")
	}
	now := time.Now().UTC()
	holder := adminUserID
	turn := turnID
	prev.Status = enum.ConversationActive
	prev.UpdatedAt = now
	prev.LastActivityAt = now
	prev.FloorHolderAdminID = &holder
	prev.FloorLastTurnAt = &now
	prev.ActiveTurnID = &turn
	prev.ActiveTurnInitiatorAdminID = &holder
	return f.store.AppendConversationMeta(ctx, prev)
}

func (f *SpeakFloor) Remaining(ctx context.Context, threadID valueobject.ThreadID) (*int64, int, error) {
	meta, ok, err := f.store.ResolveConversation(ctx, threadID)
	if err != nil {
		return nil, 0, err
	}
	if !ok {
		return nil, 0, nil
	}
	return meta.FloorHolderAdminID, f.remainingSec(meta), nil
}

func (f *SpeakFloor) remainingSec(meta entity.ConversationMeta) int {
	if meta.FloorHolderAdminID == nil || meta.FloorLastTurnAt == nil {
		return 0
	}
	ttl := float64(f.ttlSec)
	if ttl < 1 {
		ttl = 1
	}
	elapsed := time.Since(*meta.FloorLastTurnAt).Seconds()
	rem := int(math.Max(0, math.Ceil(ttl-elapsed)))
	return rem
}
