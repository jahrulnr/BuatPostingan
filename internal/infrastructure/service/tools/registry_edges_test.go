package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"buatpostingan/internal/domain/service"
	"buatpostingan/internal/infrastructure/service/docs"
	"buatpostingan/internal/infrastructure/service/tools"
)

func TestNewRegistryRequiresIndex(t *testing.T) {
	toolsRoot := filepath.Join(findRepoRoot(t), "resources", "webchat", "tools")
	_, err := tools.NewRegistry(toolsRoot, nil, tools.Options{})
	if err == nil {
		t.Fatal("nil index should fail")
	}
	_, err = tools.NewRegistry(toolsRoot, mustIndex(t), tools.Options{
		FSRoot: filepath.Join(t.TempDir(), "nope"),
	})
	if err == nil {
		t.Fatal("missing FSRoot dir should fail when set")
	}
}

func TestDocsSearchValidationAndNotReady(t *testing.T) {
	toolsRoot := filepath.Join(findRepoRoot(t), "resources", "webchat", "tools")
	docsRoot := t.TempDir()
	_ = os.WriteFile(filepath.Join(docsRoot, "a.md"), []byte("# A\nkeyword unique_abc\n"), 0o644)
	storage := t.TempDir()
	idx, err := docs.NewIndex(docsRoot, storage, docs.Options{SkipAutoBuild: true})
	if err != nil {
		t.Fatal(err)
	}
	reg, err := tools.NewRegistry(toolsRoot, idx, tools.Options{TopK: 2})
	if err != nil {
		t.Fatal(err)
	}

	empty, err := reg.Execute(context.Background(), service.ToolCall{
		Name:      "docs_search",
		Arguments: map[string]any{"query": "  "},
	})
	if err != nil {
		t.Fatal(err)
	}
	if empty.OK || empty.Error["code"] != "validation" {
		t.Fatalf("%+v", empty)
	}

	notReady, err := reg.Execute(context.Background(), service.ToolCall{
		Name:      "docs_search",
		Arguments: map[string]any{"query": "unique_abc"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if notReady.OK || notReady.Error["code"] != "docs_index_not_ready" {
		t.Fatalf("%+v", notReady)
	}

	if err := idx.Reindex(context.Background()); err != nil {
		t.Fatal(err)
	}
	ok, err := reg.Execute(context.Background(), service.ToolCall{
		Name: "docs_search",
		Arguments: map[string]any{
			"query":    "unique_abc",
			"top_k":    0,
			"language": "en",
			"domain":   "writing",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ok.OK {
		t.Fatalf("%+v", ok)
	}
}

func TestReadFileViaRegistry(t *testing.T) {
	toolsRoot := filepath.Join(findRepoRoot(t), "resources", "webchat", "tools")
	docsRoot := t.TempDir()
	_ = os.WriteFile(filepath.Join(docsRoot, "guide.md"), []byte("# Guide\nhello world\n"), 0o644)
	storage := t.TempDir()
	idx, err := docs.NewIndex(docsRoot, storage, docs.Options{})
	if err != nil {
		t.Fatal(err)
	}
	reg, err := tools.NewRegistry(toolsRoot, idx, tools.Options{FSRoot: docsRoot})
	if err != nil {
		t.Fatal(err)
	}
	env, err := reg.Execute(context.Background(), service.ToolCall{
		Name:      "read_file",
		Arguments: map[string]any{"path": "guide.md", "max_chars": "20"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !env.OK {
		t.Fatalf("%+v", env)
	}
	data, _ := env.Data.(map[string]any)
	if data["content"] == nil {
		t.Fatalf("%+v", data)
	}
}

func TestSchemasSkipsMissingToolJSON(t *testing.T) {
	docsRoot := t.TempDir()
	_ = os.WriteFile(filepath.Join(docsRoot, "a.md"), []byte("#\n"), 0o644)
	emptyTools := t.TempDir()
	storage := t.TempDir()
	idx, err := docs.NewIndex(docsRoot, storage, docs.Options{})
	if err != nil {
		t.Fatal(err)
	}
	reg, err := tools.NewRegistry(emptyTools, idx, tools.Options{})
	if err != nil {
		t.Fatal(err)
	}
	schemas, err := reg.Schemas(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(schemas) != 0 {
		t.Fatalf("want 0 schemas, got %d", len(schemas))
	}
	env, _ := reg.Execute(context.Background(), service.ToolCall{
		Name:      "list_dir",
		Arguments: nil,
	})
	if !env.OK {
		t.Fatalf("%+v", env)
	}
}

func mustIndex(t *testing.T) *docs.Index {
	t.Helper()
	docsRoot := t.TempDir()
	_ = os.WriteFile(filepath.Join(docsRoot, "a.md"), []byte("#\n"), 0o644)
	idx, err := docs.NewIndex(docsRoot, t.TempDir(), docs.Options{SkipAutoBuild: true})
	if err != nil {
		t.Fatal(err)
	}
	return idx
}
