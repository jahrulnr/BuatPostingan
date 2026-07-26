package webchat

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"buatpostingan/internal/pkg/apperr"
)

// BrowseDir lists immediate sub-directories for the workspace picker.
//
// Path resolution precedence:
//  1. Explicit non-empty path argument (absolute, as-is).
//  2. Deps.WorkspaceRoot (BP_WORKSPACE_ROOT resolved at boot).
//  3. os.Getwd() fallback.
//
// Any accessible path is listed (no jail) — the picker is operator-facing.
func (s *Service) BrowseDir(_ context.Context, path string) (BrowseDirResult, error) {
	dir := strings.TrimSpace(path)
	if dir == "" {
		dir = strings.TrimSpace(s.deps.WorkspaceRoot)
	}
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return BrowseDirResult{}, apperr.New(500, apperr.CodeInternal, "cwd unavailable: "+err.Error())
		}
		dir = cwd
	}

	st, err := os.Stat(dir)
	if err != nil {
		return BrowseDirResult{}, apperr.New(400, apperr.CodeValidation, "path not accessible")
	}
	if !st.IsDir() {
		return BrowseDirResult{}, apperr.New(400, apperr.CodeValidation, "path is not a directory")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return BrowseDirResult{}, apperr.New(400, apperr.CodeValidation, "cannot read directory")
	}

	abs, _ := filepath.Abs(dir)
	out := BrowseDirResult{Path: abs, Parent: parentDir(abs)}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		out.Entries = append(out.Entries, BrowseDirEntry{
			Name: e.Name(),
			Path: filepath.Join(abs, e.Name()),
		})
	}
	return out, nil
}

func parentDir(abs string) string {
	p := filepath.Dir(abs)
	if p == abs {
		return ""
	}
	return p
}
