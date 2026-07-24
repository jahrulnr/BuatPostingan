package llm

import "context"

type streamHooksCtxKey struct{}

// StreamHooks receive provider text deltas while Chat assembles the final payload.
// Deltas are ephemeral — callers must not treat them as durable transcript seq.
type StreamHooks struct {
	OnTextDelta func(delta string)
}

// WithStreamHooks attaches optional delta callbacks for the in-flight Chat call.
func WithStreamHooks(ctx context.Context, hooks *StreamHooks) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if hooks == nil {
		return ctx
	}
	return context.WithValue(ctx, streamHooksCtxKey{}, hooks)
}

// StreamHooksFromContext returns delta hooks when set.
func StreamHooksFromContext(ctx context.Context) *StreamHooks {
	if ctx == nil {
		return nil
	}
	hooks, _ := ctx.Value(streamHooksCtxKey{}).(*StreamHooks)
	return hooks
}
