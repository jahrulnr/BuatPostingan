package service

import (
	"context"

	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/valueobject"
)

// SpeakFloor enforces the speak-floor lock (HTTP 423) — AIPedia WebchatSpeakFloor.
type SpeakFloor interface {
	Assert(ctx context.Context, threadID valueobject.ThreadID, adminUserID int64) error
	Acquire(ctx context.Context, threadID valueobject.ThreadID, adminUserID int64, turnID valueobject.TurnID) error
	Remaining(ctx context.Context, threadID valueobject.ThreadID) (holder *int64, remainingSec int, err error)
}

// SecretRedactor scrubs API keys / JWTs from user text before enqueue/JSONL.
type SecretRedactor interface {
	Redact(ctx context.Context, text string) (redacted string, err error)
}

// DocsIndex is the lexical docs RAG index + readiness gate.
type DocsIndex interface {
	Gate(ctx context.Context) (entity.DocsIndexGate, error)
	Search(ctx context.Context, query string, topK int) (any, error)
	Reindex(ctx context.Context) error
}

// EventStreamer tails durable JSONL and emits SSE-shaped events (AIPedia WebchatEventStreamer).
// May also forward ephemeral (non-seq) events such as item.delta from an in-process hub.
type EventStreamer interface {
	Subscribe(ctx context.Context, threadID valueobject.ThreadID, afterSeq uint64, emit EventEmitFn) error
}

// EventEmitFn is one SSE application event.
type EventEmitFn func(eventName string, payload map[string]any) error

// ThreadEventHub is in-process pub/sub for SSE wakeups and ephemeral deltas.
// Durable JSONL remains the seq source of truth; hub only reduces poll latency.
type ThreadEventHub interface {
	Notify(threadID valueobject.ThreadID)
	PublishEphemeral(threadID valueobject.ThreadID, eventName string, payload map[string]any)
	Subscribe(threadID valueobject.ThreadID) ThreadEventSub
}

// ThreadEventSub is one SSE subscriber's fan-in from the hub.
type ThreadEventSub interface {
	Notify() <-chan struct{}
	Ephemeral() <-chan EphemeralEvent
	Close()
}

// EphemeralEvent is a non-durable SSE frame (no seq / Last-Event-ID).
type EphemeralEvent struct {
	Name    string
	Payload map[string]any
}

// TurnWorker runs the agent loop off the HTTP request (queue / goroutine pool).
type TurnWorker interface {
	Enqueue(ctx context.Context, job TurnJob) error
}

// TurnJob is one queued chat turn (maps to ProcessChatTurnJob::dispatch).
type TurnJob struct {
	ThreadID      valueobject.ThreadID
	TurnID        valueobject.TurnID
	AdminUserID   int64
	AdminName     string
	Message       string
	AttachmentIDs []string
	IsRetry       bool
	LockToken     string
	// ProviderID optionally pins the LLM provider for this turn (from model picker).
	ProviderID string
	// Model optionally pins the upstream model id for this turn (from model picker).
	Model string
	// Effort optionally overrides BP_LLM_EFFORT for this turn (normalized level or auto).
	Effort string
	// Workspace optionally overrides the working directory for this turn.
	// When non-empty, it replaces {{cwd}} in developer.md and becomes the default
	// base for relative paths in filesystem tools. Empty = use process cwd / config default.
	Workspace string
	// UIPath is the current browser URL/hash the user is on when starting this turn.
	// Injected into developer.md as {{ui_path}} so the agent knows which page the user views.
	UIPath string
	// TraceID correlates logs for this turn with the HTTP request that enqueued it.
	// Empty at enqueue → copied from ctx; still empty → "system".
	TraceID string
}

// ModelCatalog lists picker models and resolves per-turn model overrides (allowlist).
type ModelCatalog interface {
	ListModels(ctx context.Context) (entity.ModelsCatalog, error)
	// ResolveModel maps a picker model id or provider id to a configured provider id.
	// Empty input → empty provider (no pin). Unknown → error.
	ResolveModel(ctx context.Context, modelOrProvider string) (providerID string, err error)
}

// --- LLM / tools (used by worker later; declared here for the port map) ---

type LLMClient interface {
	Chat(ctx context.Context, messages []map[string]any, tools []map[string]any) (LLMResult, error)
}

type LLMResult struct {
	Text       string
	ToolCalls  []ToolCall
	Reasoning  string
	Model      ModelRef
	Usage      TokenUsage
	ProviderID string
	// Status is provider finish hint when present (e.g. Responses status,
	// chat finish_reason). Empty when the upstream omitted it.
	Status string
}

type ToolCall struct {
	CallID    string
	Name      string
	Arguments map[string]any
}

type ModelRef struct {
	Provider string
	ID       string
	API      string
}

type TokenUsage struct {
	InputTokens           int
	OutputTokens          int
	CachedInputTokens     int
	CacheWriteTokens      int
	ReasoningOutputTokens int
}

type LLMRouter interface {
	Chat(ctx context.Context, messages []map[string]any, tools []map[string]any, pinnedProvider string) (LLMResult, error)
}

type ToolRegistry interface {
	Schemas(ctx context.Context) ([]map[string]any, error)
	Execute(ctx context.Context, call ToolCall) (ToolEnvelope, error)
}

type ToolEnvelope struct {
	OK    bool           `json:"ok"`
	Tool  string         `json:"tool"`
	Data  any            `json:"data"`
	Error map[string]any `json:"error"`
	Meta  map[string]any `json:"meta"`
}
