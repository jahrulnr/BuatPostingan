package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"buatpostingan/internal/domain/service"
	"buatpostingan/internal/infrastructure/service/docs"
	"buatpostingan/internal/infrastructure/service/tools"
)

func TestAbsolutePathReadAllowed(t *testing.T) {
	repoRoot := findRepoRoot(t)
	docsRoot := filepath.Join(repoRoot, "docs", "webchat")
	toolsRoot := filepath.Join(repoRoot, "resources", "webchat", "tools")
	storage := t.TempDir()

	idx, err := docs.NewIndex(docsRoot, storage, docs.Options{})
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	// Empty FSRoot = unrestricted FS; use a temp file via absolute path (not host /etc).
	tmp := t.TempDir()
	absFile := filepath.Join(tmp, "note.txt")
	if err := os.WriteFile(absFile, []byte("hello absolute\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg, err := tools.NewRegistry(toolsRoot, idx, tools.Options{})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	env, err := reg.Execute(context.Background(), service.ToolCall{
		Name:      "read_file",
		Arguments: map[string]any{"path": absFile},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !env.OK {
		t.Fatalf("absolute path should be allowed: %+v", env)
	}
	data, _ := env.Data.(map[string]any)
	if data["content"] != "hello absolute\n" {
		t.Fatalf("content=%v", data["content"])
	}
}

func TestSearchDocsEnvelopeWhenIndexReady(t *testing.T) {
	repoRoot := findRepoRoot(t)
	docsRoot := filepath.Join(repoRoot, "docs", "webchat")
	toolsRoot := filepath.Join(repoRoot, "resources", "webchat", "tools")
	storage := t.TempDir()

	idx, err := docs.NewIndex(docsRoot, storage, docs.Options{})
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	reg, err := tools.NewRegistry(toolsRoot, idx, tools.Options{})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	env, err := reg.Execute(context.Background(), service.ToolCall{
		Name: "search_docs",
		Arguments: map[string]any{
			"query":  "menulis postingan judul checklist",
			"top_k":  "3", // string → healer
			"language": "id",
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !env.OK {
		t.Fatalf("expected ok envelope, got %+v", env)
	}
	if env.Tool != "search_docs" {
		t.Fatalf("tool=%s", env.Tool)
	}
	if env.Meta["data_is_untrusted"] != true {
		t.Fatalf("meta: %+v", env.Meta)
	}
	if env.Meta["index_ready"] != true {
		t.Fatalf("index_ready: %+v", env.Meta)
	}
	data, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("data type %T", env.Data)
	}
	chunks, ok := data["chunks"].([]docs.Hit)
	if !ok || len(chunks) == 0 {
		t.Fatalf("expected chunks, got %#v", data["chunks"])
	}
}

func TestArgumentHealerViaListDir(t *testing.T) {
	repoRoot := findRepoRoot(t)
	docsRoot := filepath.Join(repoRoot, "docs", "webchat")
	toolsRoot := filepath.Join(repoRoot, "resources", "webchat", "tools")
	storage := t.TempDir()

	idx, err := docs.NewIndex(docsRoot, storage, docs.Options{})
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	reg, err := tools.NewRegistry(toolsRoot, idx, tools.Options{FSRoot: docsRoot})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	env, err := reg.Execute(context.Background(), service.ToolCall{
		Name: "list_dir",
		Arguments: map[string]any{
			"path":        "writing",
			"max_entries": "10",
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !env.OK {
		t.Fatalf("list_dir failed: %+v", env)
	}
	data, _ := env.Data.(map[string]any)
	listing, _ := data["listing"].(string)
	if !strings.Contains(listing, "total ") || !strings.Contains(listing, " .") || !strings.Contains(listing, " ..") {
		t.Fatalf("expected ls-style listing with . and .., got %q", listing)
	}
	entries, _ := data["entries"].([]map[string]any)
	if len(entries) == 0 {
		t.Fatal("writing/ should have markdown entries")
	}
}

func TestListDirEmptyStillHasLSListing(t *testing.T) {
	toolsRoot := filepath.Join(findRepoRoot(t), "resources", "webchat", "tools")
	docsRoot := t.TempDir()
	empty := filepath.Join(docsRoot, "empty_dir")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	// Seed one md so docs index can construct; empty_dir itself stays empty.
	if err := os.WriteFile(filepath.Join(docsRoot, "seed.md"), []byte("# seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	storage := t.TempDir()
	idx, err := docs.NewIndex(docsRoot, storage, docs.Options{})
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	reg, err := tools.NewRegistry(toolsRoot, idx, tools.Options{FSRoot: docsRoot})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	env, err := reg.Execute(context.Background(), service.ToolCall{
		Name:      "list_dir",
		Arguments: map[string]any{"path": "empty_dir"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !env.OK {
		t.Fatalf("envelope: %+v", env)
	}
	data, _ := env.Data.(map[string]any)
	if total, _ := data["total"].(int); total != 0 {
		t.Fatalf("total=%v want 0", data["total"])
	}
	entries, _ := data["entries"].([]map[string]any)
	if len(entries) != 0 {
		t.Fatalf("entries=%#v", entries)
	}
	listing, _ := data["listing"].(string)
	if !strings.Contains(listing, "total 0") || !strings.Contains(listing, " .") || !strings.Contains(listing, " ..") {
		t.Fatalf("empty dir must still return ls-style listing, got %q", listing)
	}
}

func TestSchemasOnlyAllowlisted(t *testing.T) {
	repoRoot := findRepoRoot(t)
	docsRoot := filepath.Join(repoRoot, "docs", "webchat")
	toolsRoot := filepath.Join(repoRoot, "resources", "webchat", "tools")
	storage := t.TempDir()

	idx, err := docs.NewIndex(docsRoot, storage, docs.Options{})
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	reg, err := tools.NewRegistry(toolsRoot, idx, tools.Options{})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	schemas, err := reg.Schemas(context.Background())
	if err != nil {
		t.Fatalf("Schemas: %v", err)
	}
	if len(schemas) != len(tools.Allowlist) {
		t.Fatalf("want %d schemas, got %d", len(tools.Allowlist), len(schemas))
	}
	env, _ := reg.Execute(context.Background(), service.ToolCall{Name: "list_modules", Arguments: map[string]any{}})
	if env.OK || env.Error["code"] != "tool_not_allowed" {
		t.Fatalf("list_modules should be blocked: %+v", env)
	}
}

func findRepoRoot(t *testing.T) string {
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
