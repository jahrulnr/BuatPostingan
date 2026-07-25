package docs_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"buatpostingan/internal/infrastructure/service/docs"
)

func TestIndexBuildAndSearchFindsSeededDoc(t *testing.T) {
	repoRoot := findRepoRoot(t)
	docsRoot := filepath.Join(repoRoot, "resources", "webchat", "docs")
	storage := t.TempDir()

	idx, err := docs.NewIndex(docsRoot, storage, docs.Options{})
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}

	gate, err := idx.Gate(context.Background())
	if err != nil {
		t.Fatalf("Gate: %v", err)
	}
	if !gate.Usable || gate.Status != "ready" {
		t.Fatalf("gate not ready: %+v", gate)
	}
	if gate.DocumentCount < 1 {
		t.Fatalf("expected documents, got %d", gate.DocumentCount)
	}

	hits, err := idx.SearchHits(context.Background(), "halaman chat percakapan pesan", 5, docs.Filters{Language: "id"})
	if err != nil {
		t.Fatalf("SearchHits: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("expected at least one hit for seeded ID doc")
	}
	found := false
	for _, h := range hits {
		if filepath.Base(h.Path) == "chat-page_id.md" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("seeded doc not in hits: %+v", hits)
	}
}

func TestReindexRefreshesStatus(t *testing.T) {
	docsRoot := t.TempDir()
	_ = os.WriteFile(filepath.Join(docsRoot, "hello_id.md"), []byte("# Hello\n\nKata kunci unik xyzzy123 untuk uji.\n"), 0o644)
	storage := t.TempDir()

	idx, err := docs.NewIndex(docsRoot, storage, docs.Options{SkipAutoBuild: true})
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	gate, _ := idx.Gate(context.Background())
	if gate.Usable {
		t.Fatal("expected not usable before Reindex")
	}
	if err := idx.Reindex(context.Background()); err != nil {
		t.Fatalf("Reindex: %v", err)
	}
	gate, _ = idx.Gate(context.Background())
	if !gate.Usable {
		t.Fatalf("expected usable after Reindex: %+v", gate)
	}
	hits, _ := idx.SearchHits(context.Background(), "xyzzy123 unik", 5, docs.Filters{})
	if len(hits) == 0 {
		t.Fatal("expected hit after reindex")
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
