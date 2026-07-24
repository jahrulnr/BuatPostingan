package service

import (
	"context"

	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/valueobject"
)

// SpeakFloor enforces the 10m speak-floor lock (HTTP 423).
type SpeakFloor interface {
	Assert(ctx context.Context, threadID valueobject.ThreadID, adminUserID int64) error
	Acquire(ctx context.Context, threadID valueobject.ThreadID, adminUserID int64) error
	Remaining(ctx context.Context, threadID valueobject.ThreadID) (holder *int64, remainingSec int, err error)
}

// TurnRateLimit is a sliding-window limiter per admin (HTTP 429).
type TurnRateLimit interface {
	Assert(ctx context.Context, adminUserID int64) (retryAfterSec int, err error)
}

// DocsIndex is the lexical docs RAG index + readiness gate.
type DocsIndex interface {
	Gate(ctx context.Context) (entity.DocsIndexGate, error)
	Search(ctx context.Context, query string, topK int) (any, error)
	Reindex(ctx context.Context) error
}

// LLMClient talks to one OpenAI-compatible provider.
type LLMClient interface {
	Chat(ctx context.Context, messages []map[string]any, tools []map[string]any) (LLMResult, error)
}

// LLMResult is a normalized chat/responses outcome.
type LLMResult struct {
	Text       string
	ToolCalls  []ToolCall
	Reasoning  string
	Model      ModelRef
	Usage      TokenUsage
	ProviderID string
}

// ToolCall is a host-executed function call requested by the model.
type ToolCall struct {
	CallID    string
	Name      string
	Arguments map[string]any
}

// ModelRef is provider · model id for UI badges.
type ModelRef struct {
	Provider string
	ID       string
}

// TokenUsage is optional token accounting.
type TokenUsage struct {
	InputTokens  int
	OutputTokens int
}

// LLMRouter selects/fails over across LLMClient pool within a turn.
type LLMRouter interface {
	Chat(ctx context.Context, messages []map[string]any, tools []map[string]any, pinnedProvider string) (LLMResult, error)
}

// ToolRegistry allowlists and executes host tools (search_docs, list_dir, …).
type ToolRegistry interface {
	Schemas(ctx context.Context) ([]map[string]any, error)
	Execute(ctx context.Context, call ToolCall) (ToolEnvelope, error)
}

// ToolEnvelope is the soft-failure envelope returned to the model.
type ToolEnvelope struct {
	OK    bool           `json:"ok"`
	Tool  string         `json:"tool"`
	Data  any            `json:"data"`
	Error map[string]any `json:"error"`
	Meta  map[string]any `json:"meta"`
}

// EventStreamer tails durable items and pushes SSE-shaped events to a sink.
type EventStreamer interface {
	Subscribe(ctx context.Context, threadID valueobject.ThreadID, afterSeq uint64, emit func(eventName string, payload any) error) error
}

// TurnWorker runs the agent loop off the HTTP request (queue / goroutine pool).
type TurnWorker interface {
	Enqueue(ctx context.Context, job TurnJob) error
}

// TurnJob is one queued chat turn.
type TurnJob struct {
	ThreadID     valueobject.ThreadID
	TurnID       valueobject.TurnID
	AdminUserID  int64
	AdminName    string
	Message      string
	IsRetry      bool
}
