// Package ratelimit provides file-backed sliding-window turn rate limiting.
package ratelimit

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"buatpostingan/internal/domain/service"
	"buatpostingan/internal/pkg/apperr"
)

const (
	defaultLimitPerMin = 10
	windowSec          = 60.0
)

// TurnLimiter is a sliding-window StartTurn rate limit per admin (file-backed).
type TurnLimiter struct {
	root         string
	limitPerMin  int
}

var _ service.TurnRateLimit = (*TurnLimiter)(nil)

// NewTurnLimiter roots under storageRoot/rl/turns_{adminId}.json.
// limitPerMin defaults to 10 when <= 0.
func NewTurnLimiter(storageRoot string, limitPerMin int) *TurnLimiter {
	if limitPerMin <= 0 {
		limitPerMin = defaultLimitPerMin
	}
	return &TurnLimiter{root: storageRoot, limitPerMin: limitPerMin}
}

func (t *TurnLimiter) Assert(ctx context.Context, adminUserID int64) (int, error) {
	_ = ctx
	path := filepath.Join(t.root, "rl", "turns_"+strconv.FormatInt(adminUserID, 10)+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o775); err != nil {
		return 0, err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o664)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	if err := flockExclusive(f); err != nil {
		return 0, err
	}
	defer flockUnlock(f)

	now := float64(time.Now().UnixNano()) / 1e9
	events, err := readEvents(f)
	if err != nil {
		return 0, err
	}
	filtered := make([]float64, 0, len(events))
	for _, ts := range events {
		if now-ts < windowSec {
			filtered = append(filtered, ts)
		}
	}
	limit := t.limitPerMin
	if limit < 1 {
		limit = 1
	}
	if len(filtered) >= limit {
		oldest := filtered[0]
		for _, ts := range filtered[1:] {
			if ts < oldest {
				oldest = ts
			}
		}
		retry := int(math.Max(1, math.Ceil(windowSec-(now-oldest))))
		return retry, apperr.RateLimited(retry)
	}
	filtered = append(filtered, now)
	if err := f.Truncate(0); err != nil {
		return 0, err
	}
	if _, err := f.Seek(0, 0); err != nil {
		return 0, err
	}
	enc := json.NewEncoder(f)
	if err := enc.Encode(filtered); err != nil {
		return 0, err
	}
	return 0, nil
}

func readEvents(f *os.File) ([]float64, error) {
	buf := make([]byte, 1<<20)
	n, err := f.ReadAt(buf, 0)
	if err != nil && n == 0 {
		// empty / new file
		return nil, nil
	}
	if n == 0 {
		return nil, nil
	}
	var events []float64
	if err := json.Unmarshal(buf[:n], &events); err != nil {
		return nil, nil
	}
	return events, nil
}
