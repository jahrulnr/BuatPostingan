package ratelimit_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"buatpostingan/internal/infrastructure/ratelimit"
	"buatpostingan/internal/pkg/apperr"
)

func TestTurnLimiterDefaultLimitAndCorruptFile(t *testing.T) {
	root := t.TempDir()
	lim := ratelimit.NewTurnLimiter(root, 0) // default 10
	ctx := context.Background()
	if _, err := lim.Assert(ctx, 1); err != nil {
		t.Fatal(err)
	}

	// Corrupt events file → treated as empty, still allows
	path := filepath.Join(root, "rl", "turns_2.json")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte("not-json"), 0o644)
	if _, err := lim.Assert(ctx, 2); err != nil {
		t.Fatal(err)
	}

	// Empty file
	_ = os.WriteFile(filepath.Join(root, "rl", "turns_3.json"), []byte{}, 0o644)
	if _, err := lim.Assert(ctx, 3); err != nil {
		t.Fatal(err)
	}
}

func TestTurnLimiterUnsortedEvents(t *testing.T) {
	root := t.TempDir()
	lim := ratelimit.NewTurnLimiter(root, 2)
	ctx := context.Background()
	path := filepath.Join(root, "rl", "turns_99.json")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	now := float64(time.Now().UnixNano()) / 1e9
	// Unsorted within window — Assert picks oldest for retry
	raw, _ := json.Marshal([]float64{now - 5, now - 1, now - 30})
	_ = os.WriteFile(path, raw, 0o644)
	retry, err := lim.Assert(ctx, 99)
	if err == nil || retry < 1 {
		t.Fatalf("retry=%d err=%v", retry, err)
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeRateLimited {
		t.Fatalf("got %v", err)
	}
}

func TestTurnLimiterExpiredWindowEvents(t *testing.T) {
	root := t.TempDir()
	lim := ratelimit.NewTurnLimiter(root, 2)
	ctx := context.Background()
	path := filepath.Join(root, "rl", "turns_42.json")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	old := float64(time.Now().Add(-2*time.Minute).UnixNano()) / 1e9
	raw, _ := json.Marshal([]float64{old, old})
	_ = os.WriteFile(path, raw, 0o644)

	if _, err := lim.Assert(ctx, 42); err != nil {
		t.Fatal(err)
	}
	if _, err := lim.Assert(ctx, 42); err != nil {
		t.Fatal(err)
	}
	retry, err := lim.Assert(ctx, 42)
	if err == nil || retry < 1 {
		t.Fatalf("retry=%d err=%v", retry, err)
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeRateLimited {
		t.Fatalf("got %v", err)
	}
}

