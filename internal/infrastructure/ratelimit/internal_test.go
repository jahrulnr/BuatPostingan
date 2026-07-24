package ratelimit

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"buatpostingan/internal/pkg/apperr"
)

func TestAssertWithZeroLimitField(t *testing.T) {
	root := t.TempDir()
	// Bypass NewTurnLimiter to hit limit < 1 clamp inside Assert.
	lim := &TurnLimiter{root: root, limitPerMin: 0}
	ctx := context.Background()
	if _, err := lim.Assert(ctx, 1); err != nil {
		t.Fatal(err)
	}
	retry, err := lim.Assert(ctx, 1)
	if err == nil || retry < 1 {
		t.Fatalf("retry=%d err=%v", retry, err)
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeRateLimited {
		t.Fatalf("%v", err)
	}
}

func TestReadEventsEmptyAndArray(t *testing.T) {
	path := filepath.Join(t.TempDir(), "e.json")
	_ = os.WriteFile(path, []byte("[]"), 0o644)
	f, err := os.OpenFile(path, os.O_RDWR, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ev, err := readEvents(f)
	if err != nil || len(ev) != 0 {
		t.Fatalf("%v %#v", err, ev)
	}
	path2 := filepath.Join(t.TempDir(), "empty.json")
	f2, err := os.OpenFile(path2, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f2.Close()
	ev, err = readEvents(f2)
	if err != nil || len(ev) != 0 {
		t.Fatalf("%v %#v", err, ev)
	}
}

func TestAssertOpenFileFails(t *testing.T) {
	root := t.TempDir()
	// Make the rate-limit path a directory so OpenFile(O_RDWR|O_CREATE) fails.
	path := filepath.Join(root, "rl", "turns_7.json")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	lim := &TurnLimiter{root: root, limitPerMin: 5}
	_, err := lim.Assert(context.Background(), 7)
	if err == nil {
		t.Fatal("expected open fail")
	}
}

