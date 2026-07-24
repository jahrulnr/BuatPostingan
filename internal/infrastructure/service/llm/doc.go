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
	// Vision is BP_LLM_VISION (auto|on|off).
	Vision string
	// Effort is BP_LLM_EFFORT (auto|none|minimal|low|medium|high|xhigh|max).
	Effort string
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
		Vision:                  config.ParseVisionMode(cfg.LLMVision),
		Effort:                  config.ParseEffortMode(cfg.LLMEffort),
	}
}

// Ensure domain interfaces are satisfied by concrete types.
var (
	_ service.LLMClient = (*Client)(nil)
	_ service.LLMRouter = (*Router)(nil)
)
