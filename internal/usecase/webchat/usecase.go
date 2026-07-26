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
	DeleteThread(ctx context.Context, threadID valueobject.ThreadID) error
	StartTurn(ctx context.Context, in StartTurnInput) (StartTurnResult, error)
	RetryTurn(ctx context.Context, in RetryTurnInput) (StartTurnResult, error)
	InterruptTurn(ctx context.Context, threadID valueobject.ThreadID, turnID valueobject.TurnID, adminUserID int64) error
	UploadAttachment(ctx context.Context, in UploadAttachmentInput) (entity.AttachmentMeta, error)
	ListAttachments(ctx context.Context, threadID valueobject.ThreadID) ([]entity.AttachmentMeta, error)
	ListModels(ctx context.Context) (entity.ModelsCatalog, error)
	// BrowseDir lists sub-directories for the workspace picker UI. Empty path
	// resolves to the service default (BP_WORKSPACE_ROOT or process cwd).
	BrowseDir(ctx context.Context, path string) (BrowseDirResult, error)
	// SubscribeEvents blocks until ctx is done or the stream ends.
	// emit must be called with FE/SSE event names (item.completed, turn.*, …).
	SubscribeEvents(ctx context.Context, threadID valueobject.ThreadID, afterSeq uint64, emit EventEmitter) error
}

// BrowseDirResult is the directory listing returned to the workspace picker.
type BrowseDirResult struct {
	Path    string
	Parent  string
	Entries []BrowseDirEntry
}

// BrowseDirEntry is one sub-directory row.
type BrowseDirEntry struct {
	Name string
	Path string
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
	ThreadID      valueobject.ThreadID
	Message       string
	AdminUserID   int64
	AdminName     string
	AttachmentIDs []string
	// Model is an optional picker override (model id or provider id); validated against allowlist.
	Model string
	// Effort is an optional per-turn reasoning effort override (auto|none|…|max).
	Effort string
	// Workspace optionally overrides the working directory for this turn.
	// Empty = use config default (process cwd / FSRoot).
	Workspace string
	// UIPath is the current browser URL/hash the user is viewing.
	UIPath string
}

type RetryTurnInput struct {
	ThreadID    valueobject.ThreadID
	TurnID      valueobject.TurnID
	AdminUserID int64
	// Model is an optional picker override (model id or provider id); validated against allowlist.
	Model string
	// Effort is an optional per-turn reasoning effort override (auto|none|…|max).
	Effort string
	// Workspace optionally overrides the working directory for this turn.
	// Empty = use config default (process cwd / FSRoot).
	Workspace string
	// UIPath is the current browser URL/hash the user is viewing.
	UIPath string
}

type UploadAttachmentInput struct {
	ThreadID    valueobject.ThreadID
	Filename    string
	Mime        string
	Data        []byte
	AdminUserID int64
}

type StartTurnResult struct {
	ThreadID           valueobject.ThreadID
	TurnID             valueobject.TurnID
	SeqHead            uint64
	Status             string
	FloorHolderAdminID *int64
	FloorRemainingSec  int
}
