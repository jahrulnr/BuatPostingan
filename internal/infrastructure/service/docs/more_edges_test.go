package docs

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRankDocumentsEmptyChunksAndTopK(t *testing.T) {
	docs := []document{{
		Path:     "x.md",
		Title:    "X",
		Language: "en",
		Domain:   "general",
		Text:     "unique_rank_token_zzz appears here",
		Chunks:   nil, // forces doc-level chunk
	}}
	hits := rankDocuments(docs, "unique_rank_token_zzz", 0, Filters{}, Options{}.withDefaults())
	if len(hits) != 1 {
		t.Fatalf("%#v", hits)
	}
	hits = rankDocuments(docs, "unique_rank_token_zzz", 1, Filters{}, Options{MinScore: 999}.withDefaults())
	if len(hits) != 0 {
		t.Fatalf("min score filter: %#v", hits)
	}
}

func TestSearchErrorPathViaUnusable(t *testing.T) {
	idx, err := NewIndex(t.TempDir(), t.TempDir(), Options{SkipAutoBuild: true})
	if err != nil {
		t.Fatal(err)
	}
	any, err := idx.Search(context.Background(), "q", 1)
	if err != nil {
		t.Fatal(err)
	}
	if hits, _ := any.([]Hit); len(hits) != 0 {
		t.Fatalf("%#v", hits)
	}
}

func TestCollectDocumentsSkipsNonMarkdownAndSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "a.md"), []byte("# A\n\nbody\n"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "b.txt"), []byte("skip\n"), 0o644)
	outside := t.TempDir()
	_ = os.WriteFile(filepath.Join(outside, "secret.md"), []byte("# S\n"), 0o644)
	if err := os.Symlink(filepath.Join(outside, "secret.md"), filepath.Join(root, "escape.md")); err != nil {
		t.Skip(err)
	}
	docs, err := collectDocuments(root, "app")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range docs {
		if d.Path == "escape.md" {
			t.Fatal("symlink escape indexed")
		}
	}
	if len(docs) != 1 {
		t.Fatalf("docs=%d", len(docs))
	}
}

func TestStatusReadyWhenIndexExistsWithoutStatus(t *testing.T) {
	docsRoot := t.TempDir()
	_ = os.WriteFile(filepath.Join(docsRoot, "a.md"), []byte("# A\n"), 0o644)
	storage := t.TempDir()
	idx, err := NewIndex(docsRoot, storage, Options{})
	if err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(idx.statusPath())
	if idx.status() != "ready" {
		t.Fatalf("status=%s", idx.status())
	}
	gate, _ := idx.Gate(context.Background())
	if !gate.Usable {
		t.Fatalf("%+v", gate)
	}
}

func TestFailedGateUsesDefaultMessage(t *testing.T) {
	idx, err := NewIndex(t.TempDir(), t.TempDir(), Options{SkipAutoBuild: true})
	if err != nil {
		t.Fatal(err)
	}
	_ = idx.writeStatusLocked(statusMeta{Status: "failed"})
	gate, _ := idx.Gate(context.Background())
	if gate.Status != "failed" || gate.Message == "" {
		t.Fatalf("%+v", gate)
	}
}

func TestMinHelper(t *testing.T) {
	if min(2, 1) != 1 {
		t.Fatal("min")
	}
}
