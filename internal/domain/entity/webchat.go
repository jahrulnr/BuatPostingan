package entity

import (
	"time"

	"buatpostingan/internal/domain/enum"
	"buatpostingan/internal/domain/valueobject"
)

// TranscriptItem is one durable JSONL line (source of truth for SSE).
type TranscriptItem struct {
	Seq      uint64
	ID       valueobject.ItemID
	ThreadID valueobject.ThreadID
	TurnID   valueobject.TurnID
	Type     enum.ItemType
	// Payload holds type-specific fields (text, tool envelope, error, model, …).
	// Kept as map until typed payloads are introduced per item type.
	Payload map[string]any
	At      time.Time
}

// ConversationMeta is session-index row (sidebar + floor/active-turn state).
type ConversationMeta struct {
	ThreadID                   valueobject.ThreadID
	Title                      *valueobject.Title
	TitleSource                enum.TitleSource
	Status                     enum.ConversationStatus
	CreatedByAdminUserID       int64
	UpdatedAt                  time.Time
	LastActivityAt             time.Time
	FloorHolderAdminID         *int64
	FloorLastTurnAt            *time.Time
	ActiveTurnID               *valueobject.TurnID
	ActiveTurnInitiatorAdminID *int64
}

// ThreadSnapshot is hydrate payload for GET /threads/{id}.
type ThreadSnapshot struct {
	ThreadID                    valueobject.ThreadID
	SeqHead                     uint64
	Busy                        bool
	FloorHolderAdminID          *int64
	FloorRemainingSec           int
	ActiveTurnID                *valueobject.TurnID
	ActiveTurnInitiatorAdminID  *int64
	Items                       []TranscriptItem
}

// DocsIndexGate controls whether AI turns are allowed.
type DocsIndexGate struct {
	Usable        bool
	Status        string
	Message       string
	DocumentCount int
}
