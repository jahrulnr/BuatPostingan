package repository

import (
	"context"

	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/valueobject"
)

// ThreadStore persists JSONL transcripts + session index.
// Implementation lives under infrastructure/repository/jsonl.
type ThreadStore interface {
	CreateThread(ctx context.Context, createdByAdminUserID int64) (entity.ThreadSnapshot, error)
	GetThread(ctx context.Context, threadID valueobject.ThreadID, afterSeq uint64) (entity.ThreadSnapshot, error)
	AppendItem(ctx context.Context, threadID valueobject.ThreadID, item entity.TranscriptItem) (entity.TranscriptItem, error)
	ListConversations(ctx context.Context) ([]entity.ConversationMeta, error)
	RenameThread(ctx context.Context, threadID valueobject.ThreadID, title valueobject.Title) error
	SoftDeleteThread(ctx context.Context, threadID valueobject.ThreadID) error
	SeqHead(ctx context.Context, threadID valueobject.ThreadID) (uint64, error)
}

// ThreadLock guards one active turn per thread (HTTP 409 busy).
type ThreadLock interface {
	TryAcquire(ctx context.Context, threadID valueobject.ThreadID) (release func(), err error)
}

// InterruptFlag stores initiator-only stop requests for a turn.
type InterruptFlag interface {
	Request(ctx context.Context, threadID valueobject.ThreadID, turnID valueobject.TurnID) error
	IsRequested(ctx context.Context, threadID valueobject.ThreadID, turnID valueobject.TurnID) (bool, error)
	Clear(ctx context.Context, threadID valueobject.ThreadID, turnID valueobject.TurnID) error
}
