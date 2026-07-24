package ratelimit_test

import (
	"context"
	"testing"

	"buatpostingan/internal/infrastructure/ratelimit"
	"buatpostingan/internal/pkg/apperr"
)

func TestTurnLimiterSlidingWindow(t *testing.T) {
	root := t.TempDir()
	lim := ratelimit.NewTurnLimiter(root, 3)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		retry, err := lim.Assert(ctx, 7)
		if err != nil {
			t.Fatalf("assert %d: retry=%d err=%v", i, retry, err)
		}
	}
	retry, err := lim.Assert(ctx, 7)
	if err == nil {
		t.Fatal("expected rate limited")
	}
	if retry < 1 {
		t.Fatalf("retry=%d", retry)
	}
	ae, ok := apperr.As(err)
	if !ok || ae.Code != apperr.CodeRateLimited {
		t.Fatalf("want rate_limited got %v", err)
	}

	// Different admin is independent.
	if _, err := lim.Assert(ctx, 8); err != nil {
		t.Fatal(err)
	}
}
