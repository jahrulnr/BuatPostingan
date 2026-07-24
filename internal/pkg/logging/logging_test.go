package logging_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"buatpostingan/internal/pkg/apperr"
	"buatpostingan/internal/pkg/logging"
)

func TestNewTraceIDPrefix(t *testing.T) {
	t.Parallel()
	id := logging.NewTraceID()
	if len(id) < 4 || id[:3] != "tr_" {
		t.Fatalf("got %q", id)
	}
}

func TestWithTraceIDRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := logging.WithTraceID(context.Background(), "tr_abc")
	if got := logging.TraceID(ctx); got != "tr_abc" {
		t.Fatalf("got %q", got)
	}
}

func TestEnsureTraceIDPreserves(t *testing.T) {
	t.Parallel()
	ctx := logging.WithTraceID(context.Background(), "tr_keep")
	ctx2, id := logging.EnsureTraceID(ctx)
	if id != "tr_keep" || logging.TraceID(ctx2) != "tr_keep" {
		t.Fatalf("id=%q", id)
	}
}

func TestSystemContext(t *testing.T) {
	t.Parallel()
	ctx := logging.SystemContext(context.Background())
	if got := logging.TraceID(ctx); got != logging.TraceSystem {
		t.Fatalf("got %q want %q", got, logging.TraceSystem)
	}
}

func TestResolveIncoming(t *testing.T) {
	t.Parallel()
	if got := logging.ResolveIncoming("  my-id  ", ""); got != "my-id" {
		t.Fatalf("x-trace-id: %q", got)
	}
	tp := "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
	if got := logging.ResolveIncoming("", tp); got != "0af7651916cd43dd8448eb211c80319c" {
		t.Fatalf("traceparent: %q", got)
	}
	if got := logging.ResolveIncoming("prefer-me", tp); got != "prefer-me" {
		t.Fatalf("prefer header: %q", got)
	}
	if got := logging.ResolveIncoming("", "bad"); got != "" {
		t.Fatalf("bad tp: %q", got)
	}
}

func TestSanitizeStripsControls(t *testing.T) {
	t.Parallel()
	if got := logging.Sanitize("a\nb\x00c"); got != "abc" {
		t.Fatalf("%q", got)
	}
}

func TestBoundaryHTTPErrorSkips4xx(t *testing.T) {
	t.Parallel()
	// Smoke: must not panic; 4xx path returns early without requiring log capture.
	logging.BoundaryHTTPError(context.Background(), "op", apperr.New(http.StatusUnprocessableEntity, apperr.CodeValidation, "bad"))
	logging.BoundaryHTTPError(context.Background(), "op", errors.New("boom"))
	logging.BoundaryHTTPError(context.Background(), "op", nil)
}
