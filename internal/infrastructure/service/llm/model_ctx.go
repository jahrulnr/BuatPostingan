package llm

import (
	"context"
	"strings"
)

type modelOverrideCtxKey struct{}

// WithModelOverride attaches a per-turn upstream model id override for Client.ChatWithProvider.
func WithModelOverride(ctx context.Context, modelID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return ctx
	}
	return context.WithValue(ctx, modelOverrideCtxKey{}, modelID)
}

// ModelOverrideFromContext returns a per-turn model id override when set.
func ModelOverrideFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	v, ok := ctx.Value(modelOverrideCtxKey{}).(string)
	if !ok || strings.TrimSpace(v) == "" {
		return "", false
	}
	return strings.TrimSpace(v), true
}
