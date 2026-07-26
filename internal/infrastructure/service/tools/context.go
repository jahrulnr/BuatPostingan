package tools

import (
	"context"

	"buatpostingan/internal/domain/valueobject"
)

type threadIDCtxKey struct{}

// WithThreadID scopes attachment tools to a thread for the duration of Execute.
func WithThreadID(ctx context.Context, id valueobject.ThreadID) context.Context {
	return context.WithValue(ctx, threadIDCtxKey{}, id)
}

func threadIDFrom(ctx context.Context) (valueobject.ThreadID, bool) {
	if ctx == nil {
		return "", false
	}
	id, ok := ctx.Value(threadIDCtxKey{}).(valueobject.ThreadID)
	return id, ok && id.String() != ""
}

type workspaceCtxKey struct{}

// WithWorkspace overrides the filesystem base for relative paths in this turn.
// Empty workspace = use the registry default (FSRoot or process cwd).
func WithWorkspace(ctx context.Context, workspace string) context.Context {
	return context.WithValue(ctx, workspaceCtxKey{}, workspace)
}

// WorkspaceFromContext exposes the workspace override stored via WithWorkspace.
// Returns ("", true) when the key exists but is empty. Returns ("", false) when
// no override was set. Useful for tests/debugging.
func WorkspaceFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	ws, ok := ctx.Value(workspaceCtxKey{}).(string)
	return ws, ok
}

func workspaceFrom(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	ws, _ := ctx.Value(workspaceCtxKey{}).(string)
	return ws
}
