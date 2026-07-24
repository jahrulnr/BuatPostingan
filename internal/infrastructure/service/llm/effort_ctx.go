package llm

import (
	"context"
	"strings"
)

type effortModeCtxKey struct{}

// WithEffortMode attaches a per-turn effort override (auto|none|…|max) for Client.applyEffort.
func WithEffortMode(ctx context.Context, mode string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	mode = strings.TrimSpace(mode)
	if mode == "" {
		return ctx
	}
	return context.WithValue(ctx, effortModeCtxKey{}, mode)
}

// EffortModeFromContext returns a per-turn effort override when set.
func EffortModeFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	v, ok := ctx.Value(effortModeCtxKey{}).(string)
	if !ok || strings.TrimSpace(v) == "" {
		return "", false
	}
	return strings.TrimSpace(v), true
}
