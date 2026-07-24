package llm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type providerState struct {
	Failures int      `json:"failures"`
	OpenedAt *float64 `json:"opened_at"`
}

type circuitStore struct {
	root               string
	failureThreshold   int
	cooldownSec        int
	mu                 sync.Mutex
}

func newCircuitStore(storageRoot string, failureThreshold, cooldownSec int) *circuitStore {
	if failureThreshold < 1 {
		failureThreshold = 3
	}
	if cooldownSec < 1 {
		cooldownSec = 60
	}
	return &circuitStore{
		root:             storageRoot,
		failureThreshold: failureThreshold,
		cooldownSec:      cooldownSec,
	}
}

func (c *circuitStore) path() string {
	return filepath.Join(c.root, "llm", "provider_state.json")
}

func (c *circuitStore) read() map[string]providerState {
	raw, err := os.ReadFile(c.path())
	if err != nil {
		return map[string]providerState{}
	}
	var m map[string]providerState
	if json.Unmarshal(raw, &m) != nil || m == nil {
		return map[string]providerState{}
	}
	return m
}

func (c *circuitStore) isAvailable(providerID string, state map[string]providerState, now time.Time) bool {
	st, ok := state[providerID]
	if !ok || st.OpenedAt == nil || *st.OpenedAt <= 0 {
		return true
	}
	return now.Sub(time.Unix(0, int64(*st.OpenedAt*1e9))) >= time.Duration(c.cooldownSec)*time.Second
}

func (c *circuitStore) record(providerID string, success bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(c.path()), 0o775); err != nil {
		return
	}
	state := c.read()
	cur := state[providerID]
	if success {
		state[providerID] = providerState{Failures: 0, OpenedAt: nil}
	} else {
		failures := cur.Failures + 1
		var opened *float64
		if failures >= c.failureThreshold {
			now := float64(time.Now().UnixNano()) / 1e9
			opened = &now
		} else if cur.OpenedAt != nil {
			opened = cur.OpenedAt
		}
		state[providerID] = providerState{Failures: failures, OpenedAt: opened}
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return
	}
	_ = os.WriteFile(c.path(), append(raw, '\n'), 0o664)
}
