package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"buatpostingan/internal/domain/service"
	"buatpostingan/internal/infrastructure/service/docs"
)

func TestGrepRegexMatchAndEmpty(t *testing.T) {
	docsRoot := t.TempDir()
	mustWrite(t, filepath.Join(docsRoot, "a.md"), "# Title\nfoo bar baz\nregex_token_42\n")
	mustWrite(t, filepath.Join(docsRoot, "b.md"), "nothing here\n")
	mustWrite(t, filepath.Join(docsRoot, "skip.txt"), "regex_token_42\n")

	fs, err := newWorkspaceFS(docsRoot)
	if err != nil {
		t.Fatal(err)
	}

	// Force Go engine so tests don't depend on host rg.
	prev := findRipgrep
	findRipgrep = func(string) (string, error) { return "", os.ErrNotExist }
	t.Cleanup(func() { findRipgrep = prev })

	out, err := fs.grep(context.Background(), map[string]any{
		"query":       `regex_token_\d+`,
		"path":        "",
		"max_results": 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("envelope: %+v", out)
	}
	data := out["data"].(map[string]any)
	if data["engine"] != "go" {
		t.Fatalf("engine=%v want go", data["engine"])
	}
	matches := data["matches"].([]map[string]any)
	// Full FS: both .md and .txt match.
	if len(matches) != 2 {
		t.Fatalf("matches=%#v want 2", matches)
	}

	empty, err := fs.grep(context.Background(), map[string]any{"query": "zzz_no_such", "path": ""})
	if err != nil {
		t.Fatal(err)
	}
	edata := empty["data"].(map[string]any)
	if edata["count"] != 0 {
		t.Fatalf("empty count=%v", edata["count"])
	}
	em := edata["matches"].([]map[string]any)
	if len(em) != 0 {
		t.Fatalf("matches=%#v", em)
	}
}

func TestGrepInvalidPattern(t *testing.T) {
	docsRoot := t.TempDir()
	mustWrite(t, filepath.Join(docsRoot, "a.md"), "hello\n")
	fs, err := newWorkspaceFS(docsRoot)
	if err != nil {
		t.Fatal(err)
	}
	prev := findRipgrep
	findRipgrep = func(string) (string, error) { return "", os.ErrNotExist }
	t.Cleanup(func() { findRipgrep = prev })

	out, err := fs.grep(context.Background(), map[string]any{"query": "(", "path": ""})
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := out["ok"].(bool); ok {
		t.Fatalf("expected validation failure, got %+v", out)
	}
	errObj := out["error"].(map[string]any)
	if errObj["code"] != "validation" {
		t.Fatalf("error=%+v", errObj)
	}
}

func TestGrepAbsolutePathAllowed(t *testing.T) {
	repoRoot := findRepoRootTools(t)
	docsRoot := filepath.Join(repoRoot, "docs", "webchat")
	toolsRoot := filepath.Join(repoRoot, "resources", "webchat", "tools")
	storage := t.TempDir()
	idx, err := docs.NewIndex(docsRoot, storage, docs.Options{})
	if err != nil {
		t.Fatal(err)
	}
	reg, err := NewRegistry(toolsRoot, idx, Options{})
	if err != nil {
		t.Fatal(err)
	}

	tmp := t.TempDir()
	mustWrite(t, filepath.Join(tmp, "hit.txt"), "unique_grep_token_99\n")
	abs := filepath.Join(tmp, "hit.txt")

	env, err := reg.Execute(context.Background(), service.ToolCall{
		Name:      "grep",
		Arguments: map[string]any{"query": "unique_grep_token_99", "path": abs},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !env.OK {
		t.Fatalf("absolute grep path should work: %+v", env)
	}
	data, _ := env.Data.(map[string]any)
	if data["count"] != 1 {
		t.Fatalf("%+v", data)
	}
}

func TestGrepRipgrepEngineWhenAvailable(t *testing.T) {
	rg, err := execLookPath("rg")
	if err != nil {
		t.Skip("rg not on PATH")
	}
	docsRoot := t.TempDir()
	mustWrite(t, filepath.Join(docsRoot, "hit.md"), "alpha beta gamma\n")
	fs, err := newWorkspaceFS(docsRoot)
	if err != nil {
		t.Fatal(err)
	}
	prev := findRipgrep
	findRipgrep = func(string) (string, error) { return rg, nil }
	t.Cleanup(func() { findRipgrep = prev })

	out, err := fs.grep(context.Background(), map[string]any{
		"query": "be.a", // regex: matches "beta"
		"path":  "",
	})
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("envelope: %+v", out)
	}
	data := out["data"].(map[string]any)
	if data["engine"] != "ripgrep" {
		t.Fatalf("engine=%v", data["engine"])
	}
	matches := data["matches"].([]map[string]any)
	if len(matches) != 1 {
		t.Fatalf("matches=%#v", matches)
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func findRepoRootTools(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("go.mod not found")
	return ""
}

// execLookPath is a local alias so the ripgrep availability test stays readable.
var execLookPath = findRipgrep
