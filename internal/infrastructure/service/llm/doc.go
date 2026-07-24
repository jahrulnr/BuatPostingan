// Package llm provides OpenAI-compatible chat client + failover router.
package llm

import (
	"buatpostingan/internal/config"
	"buatpostingan/internal/domain/service"
)

// Config subset used by client/router (from config.Config).
type Config struct {
	StorageRoot                string
	Strategy                   string
	ActiveProvider             string
	TotalAttemptBudget         int
	CircuitFailureThreshold    int
	CircuitCooldownSec         int
	RetryStatuses              []int
	Providers                  map[string]config.LLMProvider
}

func FromApp(cfg config.Config) Config {
	return Config{
		StorageRoot:             cfg.StorageRoot,
		Strategy:                cfg.LLMStrategy,
		ActiveProvider:          cfg.LLMActiveProvider,
		TotalAttemptBudget:      cfg.LLMTotalAttemptBudget,
		CircuitFailureThreshold: cfg.LLMCircuitFailureThreshold,
		CircuitCooldownSec:      cfg.LLMCircuitCooldownSec,
		RetryStatuses:           cfg.LLMRetryStatuses,
		Providers:               cfg.LLMProviders,
	}
}

// Ensure domain interfaces are satisfied by concrete types.
var (
	_ service.LLMClient = (*Client)(nil)
	_ service.LLMRouter = (*Router)(nil)
)
