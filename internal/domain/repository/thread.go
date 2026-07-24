package repository

import (
	"context"

	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/valueobject"
)

// ThreadStore persists JSONL transcripts + session index (AIPedia WebchatJsonlStore).
// Implementation: infrastructure/repository/jsonl.
type ThreadStore interface {
	CreateThread(ctx context.Context, createdByAdminUserID int64) (entity.ThreadSnapshot, error)
	GetThread(ctx context.Context, threadID valueobject.ThreadID, afterSeq uint64) (entity.ThreadSnapshot, error)
	AppendItem(ctx context.Context, threadID valueobject.ThreadID, item entity.TranscriptItem) (entity.TranscriptItem, error)
	ListConversations(ctx context.Context) ([]entity.ConversationMeta, error)
	RenameThread(ctx context.Context, threadID valueobject.ThreadID, title valueobject.Title) error
	SoftDeleteThread(ctx context.Context, threadID valueobject.ThreadID) error
	SeqHead(ctx context.Context, threadID valueobject.ThreadID) (uint64, error)

	// Session-index helpers (SpeakFloor / worker active-turn).
	ResolveConversation(ctx context.Context, threadID valueobject.ThreadID) (entity.ConversationMeta, bool, error)
	AppendConversationMeta(ctx context.Context, meta entity.ConversationMeta) error
	ClearActiveTurn(ctx context.Context, threadID valueobject.ThreadID) error
}

// ThreadLock guards one active turn per thread (HTTP 409 busy).
// HTTP acquires a token and passes it to TurnWorker; worker releases when done.
type ThreadLock interface {
	TryAcquire(ctx context.Context, threadID valueobject.ThreadID) (lockToken string, err error)
	Release(ctx context.Context, threadID valueobject.ThreadID, lockToken string) error
	IsBusy(ctx context.Context, threadID valueobject.ThreadID) (bool, error)
}

// InterruptFlag stores initiator-only stop requests for a turn.
type InterruptFlag interface {
	Request(ctx context.Context, threadID valueobject.ThreadID, turnID valueobject.TurnID) error
	IsRequested(ctx context.Context, threadID valueobject.ThreadID, turnID valueobject.TurnID) (bool, error)
	Clear(ctx context.Context, threadID valueobject.ThreadID, turnID valueobject.TurnID) error
}
