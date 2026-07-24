// Package logging provides lightweight slog helpers with request/job trace IDs.
//
// Trace rules:
//   - HTTP: middleware accepts X-Trace-Id or W3C traceparent, else generates tr_<hex>.
//   - HTTP-spawned work (StartTurn → worker): propagate the request trace ID via
//     TurnJob.TraceID / context (never invent a new id in the goroutine).
//   - System-initiated work (startup, reindex, orphan recovery): use TraceSystem
//     ("system") via SystemContext — see docs/architecture/observability.md.
package logging

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"

	"buatpostingan/internal/pkg/apperr"
)

// TraceSystem is the literal trace id for pure system-initiated work
// (startup, reflex, cron-like, orphan recovery). Not used for HTTP-spawned jobs.
const TraceSystem = "system"

type ctxKey int

const traceKey ctxKey = 1

var (
	defaultLogger     *slog.Logger
	defaultLoggerOnce sync.Once
)

func baseLogger() *slog.Logger {
	defaultLoggerOnce.Do(func() {
		defaultLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	})
	return defaultLogger
}

// NewTraceID returns a fresh request/job id: tr_ + 32 hex chars.
func NewTraceID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return "tr_" + hex.EncodeToString(b[:])
}

// WithTraceID stores id on ctx (empty id is a no-op).
func WithTraceID(ctx context.Context, id string) context.Context {
	id = Sanitize(id)
	if id == "" || ctx == nil {
		return ctx
	}
	return context.WithValue(ctx, traceKey, id)
}

// TraceID returns the id from ctx, or "" if unset.
func TraceID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(traceKey).(string); ok {
		return v
	}
	return ""
}

// EnsureTraceID returns ctx with a trace id, generating one if missing.
func EnsureTraceID(ctx context.Context) (context.Context, string) {
	if ctx == nil {
		ctx = context.Background()
	}
	if id := TraceID(ctx); id != "" {
		return ctx, id
	}
	id := NewTraceID()
	return WithTraceID(ctx, id), id
}

// SystemContext returns ctx tagged with TraceSystem for non-HTTP background work.
func SystemContext(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return WithTraceID(ctx, TraceSystem)
}

// Sanitize trims and clamps a client-supplied trace id.
func Sanitize(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(id))
	for _, r := range id {
		if r < 0x20 || r == 0x7f {
			continue
		}
		b.WriteRune(r)
		if b.Len() >= 128 {
			break
		}
	}
	return b.String()
}

// ResolveIncoming picks X-Trace-Id, else the 32-hex id from W3C traceparent, else "".
func ResolveIncoming(xTraceID, traceparent string) string {
	if id := Sanitize(xTraceID); id != "" {
		return id
	}
	return parseTraceparent(traceparent)
}

func parseTraceparent(tp string) string {
	tp = strings.TrimSpace(tp)
	if tp == "" {
		return ""
	}
	parts := strings.Split(tp, "-")
	if len(parts) < 3 {
		return ""
	}
	// version-traceid-parentid-flags
	tid := parts[1]
	if len(tid) != 32 {
		return ""
	}
	for _, c := range tid {
		ok := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
		if !ok {
			return ""
		}
	}
	return strings.ToLower(tid)
}

// Logger returns a slog.Logger with trace_id bound when present.
func Logger(ctx context.Context) *slog.Logger {
	l := baseLogger()
	if id := TraceID(ctx); id != "" {
		return l.With(slog.String("trace_id", id))
	}
	return l
}

// Error logs at ERROR with op + err (and optional attrs). Prefer at boundaries only.
func Error(ctx context.Context, op string, err error, attrs ...any) {
	if err == nil {
		return
	}
	args := make([]any, 0, 4+len(attrs))
	args = append(args, slog.String("op", op), slog.String("err", err.Error()))
	args = append(args, attrs...)
	Logger(ctx).Error("error", args...)
}

// BoundaryHTTPError logs ERROR for 5xx / unknown errors at the HTTP adapter.
// Client 4xx (validation, conflict, rate limit) are not logged to avoid noise.
func BoundaryHTTPError(ctx context.Context, op string, err error) {
	if err == nil {
		return
	}
	if ae, ok := apperr.As(err); ok {
		if ae.HTTPStatus > 0 && ae.HTTPStatus < http.StatusInternalServerError {
			return
		}
		Error(ctx, op, err, slog.Int("http_status", ae.HTTPStatus), slog.String("code", string(ae.Code)))
		return
	}
	Error(ctx, op, err)
}

// Info is a thin helper for structured info lines with trace_id.
func Info(ctx context.Context, msg string, attrs ...any) {
	Logger(ctx).Info(msg, attrs...)
}

// Warn is a thin helper for structured warn lines with trace_id.
func Warn(ctx context.Context, msg string, attrs ...any) {
	Logger(ctx).Warn(msg, attrs...)
}
