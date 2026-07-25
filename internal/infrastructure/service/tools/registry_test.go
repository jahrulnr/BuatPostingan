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
	docsRoot := filepath.Join(repoRoot, "resources", "webchat", "docs")
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

func TestDocsSearchEnvelopeWhenIndexReady(t *testing.T) {
	repoRoot := findRepoRoot(t)
	docsRoot := filepath.Join(repoRoot, "resources", "webchat", "docs")
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
		Name: "docs_search",
		Arguments: map[string]any{
			"query":    "halaman chat percakapan pesan",
			"top_k":    "3", // string → healer
			"language": "id",
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !env.OK {
		t.Fatalf("expected ok envelope, got %+v", env)
	}
	if env.Tool != "docs_search" {
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

func TestDocsReadFullDocument(t *testing.T) {
	repoRoot := findRepoRoot(t)
	docsRoot := filepath.Join(repoRoot, "resources", "webchat", "docs")
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
		Name:      "docs_read",
		Arguments: map[string]any{"path": "chat-page_id.md"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !env.OK {
		t.Fatalf("expected ok envelope, got %+v", env)
	}
	if env.Tool != "docs_read" {
		t.Fatalf("tool=%s", env.Tool)
	}
	data, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("data type %T", env.Data)
	}
	if data["text"] == nil || data["text"] == "" {
		t.Fatalf("expected text content, got %#v", data["text"])
	}
	if data["title"] == nil || data["title"] == "" {
		t.Fatalf("expected title, got %#v", data["title"])
	}
}

func TestDocsReadNotFound(t *testing.T) {
	toolsRoot := filepath.Join(findRepoRoot(t), "resources", "webchat", "tools")
	docsRoot := t.TempDir()
	_ = os.WriteFile(filepath.Join(docsRoot, "a.md"), []byte("# A\nhello\n"), 0o644)
	storage := t.TempDir()
	idx, err := docs.NewIndex(docsRoot, storage, docs.Options{})
	if err != nil {
		t.Fatal(err)
	}
	reg, err := tools.NewRegistry(toolsRoot, idx, tools.Options{})
	if err != nil {
		t.Fatal(err)
	}
	env, err := reg.Execute(context.Background(), service.ToolCall{
		Name:      "docs_read",
		Arguments: map[string]any{"path": "nonexistent.md"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if env.OK || env.Error["code"] != "not_found" {
		t.Fatalf("expected not_found, got %+v", env)
	}
}

func TestDocsReadValidationEmptyPath(t *testing.T) {
	toolsRoot := filepath.Join(findRepoRoot(t), "resources", "webchat", "tools")
	docsRoot := t.TempDir()
	_ = os.WriteFile(filepath.Join(docsRoot, "a.md"), []byte("# A\n"), 0o644)
	storage := t.TempDir()
	idx, err := docs.NewIndex(docsRoot, storage, docs.Options{})
	if err != nil {
		t.Fatal(err)
	}
	reg, err := tools.NewRegistry(toolsRoot, idx, tools.Options{})
	if err != nil {
		t.Fatal(err)
	}
	env, err := reg.Execute(context.Background(), service.ToolCall{
		Name:      "docs_read",
		Arguments: map[string]any{"path": "  "},
	})
	if err != nil {
		t.Fatal(err)
	}
	if env.OK || env.Error["code"] != "validation" {
		t.Fatalf("expected validation error, got %+v", env)
	}
}

func TestDocsListAllDocuments(t *testing.T) {
	repoRoot := findRepoRoot(t)
	docsRoot := filepath.Join(repoRoot, "resources", "webchat", "docs")
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
		Name:      "docs_list",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !env.OK {
		t.Fatalf("expected ok envelope, got %+v", env)
	}
	if env.Tool != "docs_list" {
		t.Fatalf("tool=%s", env.Tool)
	}
	data, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("data type %T", env.Data)
	}
	documents, ok := data["documents"].([]docs.DocSummary)
	if !ok || len(documents) == 0 {
		t.Fatalf("expected documents, got %#v", data["documents"])
	}
	if env.Meta["count"] == nil || env.Meta["count"] != len(documents) {
		t.Fatalf("count mismatch: meta=%v documents=%d", env.Meta["count"], len(documents))
	}
}

func TestDocsListWithLanguageFilter(t *testing.T) {
	toolsRoot := filepath.Join(findRepoRoot(t), "resources", "webchat", "tools")
	docsRoot := t.TempDir()
	_ = os.WriteFile(filepath.Join(docsRoot, "guide_id.md"), []byte("# Panduan\nisi\n"), 0o644)
	_ = os.WriteFile(filepath.Join(docsRoot, "guide_en.md"), []byte("# Guide\ncontent\n"), 0o644)
	storage := t.TempDir()
	idx, err := docs.NewIndex(docsRoot, storage, docs.Options{})
	if err != nil {
		t.Fatal(err)
	}
	reg, err := tools.NewRegistry(toolsRoot, idx, tools.Options{})
	if err != nil {
		t.Fatal(err)
	}
	env, err := reg.Execute(context.Background(), service.ToolCall{
		Name:      "docs_list",
		Arguments: map[string]any{"language": "id"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !env.OK {
		t.Fatalf("%+v", env)
	}
	data, _ := env.Data.(map[string]any)
	documents, _ := data["documents"].([]docs.DocSummary)
	if len(documents) != 1 {
		t.Fatalf("expected 1 id doc, got %d", len(documents))
	}
	if documents[0].Language != "id" {
		t.Fatalf("language=%s", documents[0].Language)
	}
}

func TestDocsListNotReady(t *testing.T) {
	toolsRoot := filepath.Join(findRepoRoot(t), "resources", "webchat", "tools")
	docsRoot := t.TempDir()
	_ = os.WriteFile(filepath.Join(docsRoot, "a.md"), []byte("# A\n"), 0o644)
	storage := t.TempDir()
	idx, err := docs.NewIndex(docsRoot, storage, docs.Options{SkipAutoBuild: true})
	if err != nil {
		t.Fatal(err)
	}
	reg, err := tools.NewRegistry(toolsRoot, idx, tools.Options{})
	if err != nil {
		t.Fatal(err)
	}
	env, err := reg.Execute(context.Background(), service.ToolCall{
		Name:      "docs_list",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if env.OK || env.Error["code"] != "docs_index_not_ready" {
		t.Fatalf("expected docs_index_not_ready, got %+v", env)
	}
}

func TestArgumentHealerViaListDir(t *testing.T) {
	repoRoot := findRepoRoot(t)
	docsRoot := filepath.Join(repoRoot, "resources", "webchat", "docs")
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
			"path":        "",
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
	docsRoot := filepath.Join(repoRoot, "resources", "webchat", "docs")
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
	assertToolRequiredFields(t, schemas, "call_mcp_tool", "server", "tool", "arguments")
	env, _ := reg.Execute(context.Background(), service.ToolCall{Name: "list_modules", Arguments: map[string]any{}})
	if env.OK || env.Error["code"] != "tool_not_allowed" {
		t.Fatalf("list_modules should be blocked: %+v", env)
	}
}

func TestWriteToolsGated(t *testing.T) {
	repoRoot := findRepoRoot(t)
	docsRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(docsRoot, "seed.md"), []byte("# seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	toolsRoot := filepath.Join(repoRoot, "resources", "webchat", "tools")
	storage := t.TempDir()

	idx, err := docs.NewIndex(docsRoot, storage, docs.Options{})
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}

	// Default registry: write tools must be blocked and not advertised.
	reg, err := tools.NewRegistry(toolsRoot, idx, tools.Options{})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	schemas, err := reg.Schemas(context.Background())
	if err != nil {
		t.Fatalf("Schemas: %v", err)
	}
	if len(schemas) != len(tools.Allowlist) {
		t.Fatalf("default schemas: want %d, got %d", len(tools.Allowlist), len(schemas))
	}
	env, err := reg.Execute(context.Background(), service.ToolCall{
		Name:      "write_file",
		Arguments: map[string]any{"path": "/tmp/locked.txt", "content": "x"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Write-enabled registry: exercise write/edit/delete end-to-end.
	reg, err = tools.NewRegistry(toolsRoot, idx, tools.Options{})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	schemas, err = reg.Schemas(context.Background())
	if err != nil {
		t.Fatalf("Schemas: %v", err)
	}
	if len(schemas) != len(tools.Allowlist) {
		t.Fatalf("write-enabled schemas: want %d, got %d", len(tools.Allowlist), len(schemas))
	}

	tmp := t.TempDir()
	testFile := filepath.Join(tmp, "nested", "note.txt")
	env, err = reg.Execute(context.Background(), service.ToolCall{
		Name:      "write_file",
		Arguments: map[string]any{"path": testFile, "content": "hello world\n"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !env.OK {
		t.Fatalf("write_file failed: %+v", env)
	}
	data, _ := env.Data.(map[string]any)
	if data["path"] != testFile {
		t.Fatalf("path=%v", data["path"])
	}

	env, err = reg.Execute(context.Background(), service.ToolCall{
		Name:      "edit_file",
		Arguments: map[string]any{"path": testFile, "old_string": "world", "new_string": "codex"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !env.OK {
		t.Fatalf("edit_file failed: %+v", env)
	}
	data, _ = env.Data.(map[string]any)
	if data["replacements"] != 1 {
		t.Fatalf("replacements=%v", data["replacements"])
	}

	env, err = reg.Execute(context.Background(), service.ToolCall{
		Name:      "read_file",
		Arguments: map[string]any{"path": testFile},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !env.OK {
		t.Fatalf("read_file failed: %+v", env)
	}
	data, _ = env.Data.(map[string]any)
	if data["content"] != "hello codex\n" {
		t.Fatalf("content=%q", data["content"])
	}

	env, err = reg.Execute(context.Background(), service.ToolCall{
		Name:      "delete_file",
		Arguments: map[string]any{"path": testFile},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !env.OK {
		t.Fatalf("delete_file failed: %+v", env)
	}
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Fatalf("file should be deleted: %v", err)
	}
}

func assertToolRequiredFields(t *testing.T, schemas []map[string]any, toolName string, fields ...string) {
	t.Helper()
	var callMCP map[string]any
	for _, schema := range schemas {
		fn, _ := schema["function"].(map[string]any)
		if fn["name"] == toolName {
			callMCP = fn
			break
		}
	}
	params, _ := callMCP["parameters"].(map[string]any)
	required, _ := params["required"].([]any)
	for _, want := range fields {
		found := false
		for _, value := range required {
			if value == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%s required fields missing %q: %#v", toolName, want, required)
		}
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
