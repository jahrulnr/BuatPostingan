package enum

// ItemType is a durable JSONL / SSE item type.
type ItemType string

const (
	ItemThreadStarted  ItemType = "thread.started"
	ItemUserMessage    ItemType = "user_message"
	ItemTurnStarted    ItemType = "turn.started"
	ItemTurnResumed    ItemType = "turn.resumed"
	ItemTurnCompleted  ItemType = "turn.completed"
	ItemTurnFailed     ItemType = "turn.failed"
	ItemReasoning      ItemType = "reasoning"
	ItemToolCall       ItemType = "tool_call"
	ItemToolResult     ItemType = "tool_result"
	ItemAgentMessage   ItemType = "agent_message"
	ItemContextCompact ItemType = "context_compacted"
)

// TitleSource tracks how a conversation title was set.
type TitleSource string

const (
	TitlePending TitleSource = "pending"
	TitleAuto    TitleSource = "auto"
	TitleManual  TitleSource = "manual"
	TitleStale   TitleSource = "stale"
)

// ConversationStatus is lifecycle status in the session index.
type ConversationStatus string

const (
	ConversationActive  ConversationStatus = "active"
	ConversationDeleted ConversationStatus = "deleted"
)

// LLMStrategy selects provider routing behaviour.
type LLMStrategy string

const (
	LLMFailover    LLMStrategy = "failover"
	LLMRoundRobin  LLMStrategy = "round_robin"
	LLMSwitchFixed LLMStrategy = "switch"
)
