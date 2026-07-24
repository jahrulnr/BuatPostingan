package llm

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"buatpostingan/internal/pkg/logging"
)

// providerState is the persisted per-provider circuit record.
//
// Derived states:
//   - closed:    OpenedAt == nil (Failures may accumulate below threshold)
//   - open:      OpenedAt != nil and now-OpenedAt < cooldown  → skip provider
//   - half-open: OpenedAt != nil and now-OpenedAt >= cooldown → allow one probe
//
// ProbeAt leases the single half-open probe. A stale lease (older than probeTTL)
// is reclaimable so a crashed/hung probe never wedges a provider half-open.
type providerState struct {
	Failures int      `json:"failures"`
	OpenedAt *float64 `json:"opened_at"`
	ProbeAt  *float64 `json:"probe_at,omitempty"`
}

type circuitStore struct {
	root             string
	failureThreshold int
	cooldownSec      int
	probeTTL         time.Duration
	mu               sync.Mutex
}

func newCircuitStore(storageRoot string, failureThreshold, cooldownSec int) *circuitStore {
	if failureThreshold < 1 {
		failureThreshold = 3
	}
	if cooldownSec < 1 {
		cooldownSec = 60
	}
	probeTTL := time.Duration(cooldownSec) * time.Second
	if probeTTL < 30*time.Second {
		probeTTL = 30 * time.Second
	}
	return &circuitStore{
		root:             storageRoot,
		failureThreshold: failureThreshold,
		cooldownSec:      cooldownSec,
		probeTTL:         probeTTL,
	}
}

func (c *circuitStore) path() string {
	return filepath.Join(c.root, "llm", "provider_state.json")
}

func (c *circuitStore) lockPath() string {
	return filepath.Join(c.root, "llm", "provider_state.lock")
}

// read returns the current state, tolerating a missing or corrupt file (→ empty,
// which reads as all-closed). Lock-free: writers use atomic temp+rename so a
// reader always observes a complete document.
func (c *circuitStore) read() map[string]providerState {
	m, _ := c.readParse()
	return m
}

func (c *circuitStore) readParse() (map[string]providerState, bool) {
	raw, err := os.ReadFile(c.path())
	if err != nil {
		return map[string]providerState{}, false
	}
	var m map[string]providerState
	if json.Unmarshal(raw, &m) != nil || m == nil {
		// Corrupt / partial file → recover safely as empty (all-closed).
		return map[string]providerState{}, true
	}
	return m, false
}

func unixSeconds(t time.Time) float64 { return float64(t.UnixNano()) / 1e9 }

func secondsToTime(sec float64) time.Time { return time.Unix(0, int64(sec*1e9)) }

// isAvailable reports whether a provider may be tried (closed or half-open).
// Fully-open providers within cooldown are excluded from the candidate list.
func (c *circuitStore) isAvailable(providerID string, state map[string]providerState, now time.Time) bool {
	st, ok := state[providerID]
	if !ok || st.OpenedAt == nil || *st.OpenedAt <= 0 {
		return true
	}
	return now.Sub(secondsToTime(*st.OpenedAt)) >= time.Duration(c.cooldownSec)*time.Second
}

// withLock serializes read-modify-write across goroutines (mutex) and processes
// (flock on a sidecar lock file).
func (c *circuitStore) withLock(fn func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(c.path()), 0o775); err != nil {
		fn()
		return
	}
	lf, err := os.OpenFile(c.lockPath(), os.O_CREATE|os.O_RDWR, 0o664)
	if err == nil {
		_ = lockFile(lf)
		defer func() {
			_ = unlockFile(lf)
			_ = lf.Close()
		}()
	}
	fn()
}

// writeAtomic persists state via temp file + rename so readers never see a
// half-written document.
func (c *circuitStore) writeAtomic(state map[string]providerState) {
	raw, err := json.Marshal(state)
	if err != nil {
		return
	}
	raw = append(raw, '\n')
	dir := filepath.Dir(c.path())
	tmp, err := os.CreateTemp(dir, "provider_state-*.tmp")
	if err != nil {
		return
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return
	}
	if err := os.Rename(tmpName, c.path()); err != nil {
		_ = os.Remove(tmpName)
	}
}

// tryAcquire gates a provider before a call. Closed providers always pass
// without a write. For a half-open provider it leases the single probe: the
// first caller (across processes) wins; concurrent callers fail fast (false)
// so the router moves to an alternate provider instead of stampeding.
func (c *circuitStore) tryAcquire(ctx context.Context, providerID string) bool {
	allowed := false
	c.withLock(func() {
		state, corrupt := c.readParse()
		if corrupt {
			logging.Warn(ctx, "webchat.llm.circuit",
				"provider", providerID, "state", "corrupt_reset")
		}
		st := state[providerID]
		now := time.Now()
		if st.OpenedAt == nil || *st.OpenedAt <= 0 {
			allowed = true // closed
			return
		}
		if now.Sub(secondsToTime(*st.OpenedAt)) < time.Duration(c.cooldownSec)*time.Second {
			allowed = false // still open
			return
		}
		// Half-open: one probe at a time.
		if st.ProbeAt != nil && now.Sub(secondsToTime(*st.ProbeAt)) < c.probeTTL {
			allowed = false
			return
		}
		lease := unixSeconds(now)
		st.ProbeAt = &lease
		state[providerID] = st
		c.writeAtomic(state)
		allowed = true
		logging.Info(ctx, "webchat.llm.circuit",
			"provider", providerID, "state", "half_open_probe", "cooldown_sec", c.cooldownSec)
	})
	return allowed
}

// record folds a call result into the circuit. Only transient/provider failures
// should reach here as success=false; auth/validation errors must not trip the
// circuit and are recorded by the caller with success left untouched.
func (c *circuitStore) record(ctx context.Context, providerID string, success bool) {
	c.withLock(func() {
		state, corrupt := c.readParse()
		if corrupt {
			logging.Warn(ctx, "webchat.llm.circuit",
				"provider", providerID, "state", "corrupt_reset")
		}
		cur := state[providerID]
		now := time.Now()
		prevOpen := cur.OpenedAt != nil && *cur.OpenedAt > 0
		if success {
			if prevOpen {
				logging.Warn(ctx, "webchat.llm.circuit",
					"provider", providerID, "state", "closed", "reason", "probe_success")
			}
			state[providerID] = providerState{}
			c.writeAtomic(state)
			return
		}
		failures := cur.Failures + 1
		ns := providerState{Failures: failures, OpenedAt: cur.OpenedAt, ProbeAt: cur.ProbeAt}
		switch {
		case prevOpen:
			// Half-open probe failed → reopen with a fresh cooldown.
			opened := unixSeconds(now)
			ns.OpenedAt = &opened
			ns.ProbeAt = nil
			logging.Warn(ctx, "webchat.llm.circuit",
				"provider", providerID, "state", "open", "reason", "probe_failed",
				"failures", failures, "cooldown_sec", c.cooldownSec)
		case failures >= c.failureThreshold:
			opened := unixSeconds(now)
			ns.OpenedAt = &opened
			ns.ProbeAt = nil
			logging.Warn(ctx, "webchat.llm.circuit",
				"provider", providerID, "state", "open", "reason", "threshold",
				"failures", failures, "threshold", c.failureThreshold, "cooldown_sec", c.cooldownSec)
		}
		state[providerID] = ns
		c.writeAtomic(state)
	})
}
