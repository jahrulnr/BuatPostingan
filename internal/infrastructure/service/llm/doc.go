// Package llm provides OpenAI-compatible chat client + failover router.
package llm

import (
	"buatpostingan/internal/config"
	"buatpostingan/internal/domain/service"
)

// Config subset used by client/router (from config.Config).
type Config struct {
	StorageRoot             string
	Strategy                string
	ActiveProvider          string
	TotalAttemptBudget      int
	CircuitFailureThreshold int
	CircuitCooldownSec      int
	RetryStatuses           []int
	Providers               map[string]config.LLMProvider
	// Stream requests stream=true when non-nil. Nil defaults to true (SSE preferred).
	Stream *bool
}

func FromApp(cfg config.Config) Config {
	stream := cfg.LLMStream
	return Config{
		StorageRoot:             cfg.StorageRoot,
		Strategy:                cfg.LLMStrategy,
		ActiveProvider:          cfg.LLMActiveProvider,
		TotalAttemptBudget:      cfg.LLMTotalAttemptBudget,
		CircuitFailureThreshold: cfg.LLMCircuitFailureThreshold,
		CircuitCooldownSec:      cfg.LLMCircuitCooldownSec,
		RetryStatuses:           cfg.LLMRetryStatuses,
		Providers:               cfg.LLMProviders,
		Stream:                  &stream,
	}
}

// Ensure domain interfaces are satisfied by concrete types.
var (
	_ service.LLMClient = (*Client)(nil)
	_ service.LLMRouter = (*Router)(nil)
)
