package llm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"buatpostingan/internal/domain/service"
)

// Router fails over among enabled providers; pins after first success in a turn.
type Router struct {
	cfg     Config
	client  *Client
	circuit *circuitStore
}

func NewRouter(cfg Config, client *Client) *Router {
	if client == nil {
		client = NewClient(cfg)
	}
	return &Router{
		cfg:     cfg,
		client:  client,
		circuit: newCircuitStore(cfg.StorageRoot, cfg.CircuitFailureThreshold, cfg.CircuitCooldownSec),
	}
}

func (r *Router) Chat(ctx context.Context, messages []map[string]any, tools []map[string]any, pinnedProvider string) (service.LLMResult, error) {
	candidates := r.candidates(pinnedProvider)
	attempts := 0
	budget := r.cfg.TotalAttemptBudget
	if budget < 1 {
		budget = 4
	}
	var last error
	for _, providerID := range candidates {
		p, ok := r.cfg.Providers[providerID]
		if !ok || !p.Enabled {
			continue
		}
		maxAttempts := p.MaxAttempts
		if maxAttempts < 1 {
			maxAttempts = 1
		}
		for a := 0; a < maxAttempts && attempts < budget; a++ {
			attempts++
			res, err := r.client.ChatWithProvider(ctx, providerID, messages, tools)
			if err == nil {
				r.circuit.record(providerID, true)
				return res, nil
			}
			last = err
			r.circuit.record(providerID, false)
			le, ok := err.(*Error)
			if ok && !le.Transient {
				return service.LLMResult{}, err
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

func (r *Router) candidates(pinnedProvider string) []string {
	now := time.Now()
	state := r.circuit.read()
	var enabled []string
	for id, p := range r.cfg.Providers {
		if !p.Enabled {
			continue
		}
		if r.circuit.isAvailable(id, state, now) {
			enabled = append(enabled, id)
		}
	}
	if len(enabled) == 0 {
		for id, p := range r.cfg.Providers {
			if p.Enabled {
				enabled = append(enabled, id)
			}
		}
	}

	if r.cfg.Strategy == "switch" {
		if r.cfg.ActiveProvider != "" {
			return []string{r.cfg.ActiveProvider}
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

	if r.cfg.Strategy == "round_robin" && len(enabled) > 0 {
		enabled = r.rotateRoundRobin(enabled)
	}

	if r.cfg.Strategy == "failover" && r.cfg.ActiveProvider != "" {
		rest := make([]string, 0, len(enabled))
		for _, id := range enabled {
			if id != r.cfg.ActiveProvider {
				rest = append(rest, id)
			}
		}
		hasActive := false
		for _, id := range enabled {
			if id == r.cfg.ActiveProvider {
				hasActive = true
				break
			}
		}
		if hasActive {
			return append([]string{r.cfg.ActiveProvider}, rest...)
		}
	}
	return enabled
}

func (r *Router) rotateRoundRobin(enabled []string) []string {
	path := filepath.Join(r.cfg.StorageRoot, "llm", "round_robin.cursor")
	_ = os.MkdirAll(filepath.Dir(path), 0o775)
	cursor := 0
	if raw, err := os.ReadFile(path); err == nil {
		cursor, _ = strconv.Atoi(strings.TrimSpace(string(raw)))
	}
	if len(enabled) == 0 {
		return enabled
	}
	start := cursor % len(enabled)
	_ = os.WriteFile(path, []byte(fmt.Sprintf("%d", (start+1)%len(enabled))), 0o664)
	return append(append([]string{}, enabled[start:]...), enabled[:start]...)
}
