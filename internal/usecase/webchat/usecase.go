package webchat

import (
	"context"
	"time"

	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/valueobject"
)

// Usecase is the application port consumed by delivery/http.
// Concrete implementation lives under internal/ (later); delivery must not
// depend on infrastructure ports directly.
type Usecase interface {
	ListConversations(ctx context.Context) (ListConversationsResult, error)
	CreateThread(ctx context.Context, adminUserID int64) (CreateThreadResult, error)
	GetThread(ctx context.Context, threadID valueobject.ThreadID, afterSeq uint64) (entity.ThreadSnapshot, error)
	RenameThread(ctx context.Context, threadID valueobject.ThreadID, title valueobject.Title) (RenameResult, error)
	StartTurn(ctx context.Context, in StartTurnInput) (StartTurnResult, error)
	RetryTurn(ctx context.Context, threadID valueobject.ThreadID, turnID valueobject.TurnID, adminUserID int64) (StartTurnResult, error)
	InterruptTurn(ctx context.Context, threadID valueobject.ThreadID, turnID valueobject.TurnID, adminUserID int64) error
	// SubscribeEvents blocks until ctx is done or the stream ends.
	// emit must be called with FE/SSE event names (item.completed, turn.*, …).
	SubscribeEvents(ctx context.Context, threadID valueobject.ThreadID, afterSeq uint64, emit EventEmitter) error
}

// EventEmitter writes one SSE application event. payload should be JSON-serializable.
type EventEmitter func(eventName string, payload map[string]any) error

type ListConversationsResult struct {
	Conversations []ConversationView
	DocsIndex     entity.DocsIndexGate
}

// ConversationView is sidebar row + computed floor remaining for the caller.
type ConversationView struct {
	Meta              entity.ConversationMeta
	FloorRemainingSec int
}

type CreateThreadResult struct {
	ThreadID             valueobject.ThreadID
	SeqHead              uint64
	CreatedByAdminUserID int64
	CreatedAt            time.Time
}

type RenameResult struct {
	ThreadID valueobject.ThreadID
	Title    valueobject.Title
}

type StartTurnInput struct {
	ThreadID    valueobject.ThreadID
	Message     string
	AdminUserID int64
	AdminName   string
}

type StartTurnResult struct {
	ThreadID           valueobject.ThreadID
	TurnID             valueobject.TurnID
	SeqHead            uint64
	Status             string
	FloorHolderAdminID *int64
	FloorRemainingSec  int
}
