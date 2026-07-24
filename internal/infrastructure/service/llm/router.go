package llm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"buatpostingan/internal/domain/service"
)

// Router fails over among enabled providers; pins after first success in a turn.
type Router struct {
	mu      sync.RWMutex
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

// Reload updates router + underlying client after settings change.
func (r *Router) Reload(cfg Config) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cfg = cfg
	if r.client != nil {
		r.client.Reload(cfg)
	}
	r.circuit = newCircuitStore(cfg.StorageRoot, cfg.CircuitFailureThreshold, cfg.CircuitCooldownSec)
}

func (r *Router) Chat(ctx context.Context, messages []map[string]any, tools []map[string]any, pinnedProvider string) (service.LLMResult, error) {
	r.mu.RLock()
	cfg := r.cfg
	client := r.client
	circuit := r.circuit
	r.mu.RUnlock()
	candidates := candidatesFor(cfg, circuit, pinnedProvider)
	attempts := 0
	budget := cfg.TotalAttemptBudget
	if budget < 1 {
		budget = 4
	}
	var last error
	for _, providerID := range candidates {
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
				circuit.record(providerID, true)
				return res, nil
			}
			last = err
			circuit.record(providerID, false)
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
	r.mu.RLock()
	defer r.mu.RUnlock()
	return candidatesFor(r.cfg, r.circuit, pinnedProvider)
}

func candidatesFor(cfg Config, circuit *circuitStore, pinnedProvider string) []string {
	now := time.Now()
	state := circuit.read()
	var enabled []string
	for id, p := range cfg.Providers {
		if !p.Enabled {
			continue
		}
		if circuit.isAvailable(id, state, now) {
			enabled = append(enabled, id)
		}
	}
	if len(enabled) == 0 {
		for id, p := range cfg.Providers {
			if p.Enabled {
				enabled = append(enabled, id)
			}
		}
	}

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
	_ = os.WriteFile(path, []byte(fmt.Sprintf("%d", (start+1)%len(enabled))), 0o664)
	return append(append([]string{}, enabled[start:]...), enabled[:start]...)
}

func (r *Router) rotateRoundRobin(enabled []string) []string {
	r.mu.RLock()
	root := r.cfg.StorageRoot
	r.mu.RUnlock()
	return rotateRoundRobin(root, enabled)
}
