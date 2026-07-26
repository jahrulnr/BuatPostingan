package llm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"buatpostingan/internal/domain/service"
	"buatpostingan/internal/pkg/logging"
)

// Router fails over among enabled providers; pins after first success in a turn.
type Router struct {
	mu     sync.RWMutex
	cfg    Config
	client *Client
	retry  *retryPolicy
}

func NewRouter(cfg Config, client *Client) *Router {
	if client == nil {
		client = NewClient(cfg)
	}
	return &Router{
		cfg:    cfg,
		client: client,
		retry:  newRetryPolicy(cfg),
	}
}

// Reload updates router + underlying client after settings change.
func (r *Router) Reload(cfg Config) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cfg = cfg
	if r.client != nil {
		r.client.Reload(cfg)
	}
	r.retry = newRetryPolicy(cfg)
}

func (r *Router) Chat(ctx context.Context, messages []map[string]any, tools []map[string]any, pinnedProvider string) (service.LLMResult, error) {
	r.mu.RLock()
	cfg := r.cfg
	client := r.client
	policy := r.retry
	r.mu.RUnlock()
	candidates := candidatesFor(cfg, pinnedProvider)
	attempts := 0
	retryNum := 0
	budget := cfg.TotalAttemptBudget
	if budget < 1 {
		budget = 4
	}
	var last error
	for ci, providerID := range candidates {
		p, ok := cfg.Providers[providerID]
		if !ok || !p.Enabled {
			continue
		}
		maxAttempts := p.MaxAttempts
		if maxAttempts < 1 {
			maxAttempts = 1
		}
		for a := 0; a < maxAttempts && attempts < budget; a++ {
			attempts++
			res, err := client.ChatWithProvider(ctx, providerID, messages, tools)
			if err == nil {
				return res, nil
			}
			last = err
			le, isLE := err.(*Error)
			transient := isLE && le.Transient
			if transient {
				// Only transient/provider failures are eligible for retry/failover;
				// auth/validation errors (non-transient) stop immediately.
			} else {
				return service.LLMResult{}, err
			}
			// Another attempt coming (this provider or a later candidate)?
			moreHere := a+1 < maxAttempts && attempts < budget
			moreProviders := ci+1 < len(candidates) && attempts < budget
			if !moreHere && !moreProviders {
				break
			}
			retryNum++
			var retryAfter time.Duration
			if isLE {
				retryAfter = le.RetryAfter
			}
			delay := policy.delay(retryNum, retryAfter)
			logging.Warn(ctx, "webchat.llm.retry_backoff",
				"provider", providerID,
				"attempt", attempts,
				"max_attempts", maxAttempts,
				"budget_used", attempts,
				"budget", budget,
				"delay_ms", delay.Milliseconds(),
				"retry_after_ms", retryAfter.Milliseconds(),
				"status", statusOf(le),
				"kind", transientKind(err),
				"err", err.Error(),
			)
			if werr := policy.wait(ctx, delay); werr != nil {
				// Context cancelled/deadline: never sleep past it; surface the
				// provider error that triggered the (aborted) retry.
				return service.LLMResult{}, last
			}
		}
		if attempts >= budget {
			break
		}
	}
	if last != nil {
		return service.LLMResult{}, last
	}
	return service.LLMResult{}, &Error{Provider: "none", Msg: "LLM providers exhausted", Transient: true}
}

func statusOf(le *Error) int {
	if le == nil {
		return 0
	}
	return le.Status
}

func (r *Router) candidates(pinnedProvider string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return candidatesFor(r.cfg, pinnedProvider)
}

func candidatesFor(cfg Config, pinnedProvider string) []string {
	var enabled []string
	for id, p := range cfg.Providers {
		if !p.Enabled {
			continue
		}
		enabled = append(enabled, id)
	}
	sort.Strings(enabled)

	if cfg.Strategy == "switch" {
		if cfg.ActiveProvider != "" {
			return []string{cfg.ActiveProvider}
		}
		return enabled
	}

	if pinnedProvider != "" {
		for _, id := range enabled {
			if id == pinnedProvider {
				rest := make([]string, 0, len(enabled))
				for _, x := range enabled {
					if x != pinnedProvider {
						rest = append(rest, x)
					}
				}
				return append([]string{pinnedProvider}, rest...)
			}
		}
	}

	if cfg.Strategy == "round_robin" && len(enabled) > 0 {
		enabled = rotateRoundRobin(cfg.StorageRoot, enabled)
	}

	if cfg.Strategy == "failover" && cfg.ActiveProvider != "" {
		rest := make([]string, 0, len(enabled))
		for _, id := range enabled {
			if id != cfg.ActiveProvider {
				rest = append(rest, id)
			}
		}
		hasActive := false
		for _, id := range enabled {
			if id == cfg.ActiveProvider {
				hasActive = true
				break
			}
		}
		if hasActive {
			return append([]string{cfg.ActiveProvider}, rest...)
		}
	}
	return enabled
}

func rotateRoundRobin(storageRoot string, enabled []string) []string {
	path := filepath.Join(storageRoot, "llm", "round_robin.cursor")
	_ = os.MkdirAll(filepath.Dir(path), 0o775)
	cursor := 0
	if raw, err := os.ReadFile(path); err == nil {
		cursor, _ = strconv.Atoi(strings.TrimSpace(string(raw)))
	}
	if len(enabled) == 0 {
		return enabled
	}
	start := cursor % len(enabled)
	_ = os.WriteFile(path, fmt.Appendf(nil, "%d", (start+1)%len(enabled)), 0o664)
	return append(append([]string{}, enabled[start:]...), enabled[:start]...)
}

func (r *Router) rotateRoundRobin(enabled []string) []string {
	r.mu.RLock()
	root := r.cfg.StorageRoot
	r.mu.RUnlock()
	return rotateRoundRobin(root, enabled)
}
