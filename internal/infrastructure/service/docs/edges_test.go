package docs

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewIndexValidation(t *testing.T) {
	_, err := NewIndex("", t.TempDir(), Options{})
	if err == nil {
		t.Fatal("docsRoot required")
	}
	_, err = NewIndex(t.TempDir(), "", Options{})
	if err == nil {
		t.Fatal("storageRoot required")
	}
}

func TestSearchAndGateStatuses(t *testing.T) {
	docsRoot := t.TempDir()
	longBody := "# Title\n\n" + strings.Repeat("unique_word_xyz ", 40) + "\n## Section\n\nNo query match filler text for snippet truncate path.\n"
	_ = os.WriteFile(filepath.Join(docsRoot, "doc_en.md"), []byte(longBody), 0o644)
	_ = os.WriteFile(filepath.Join(docsRoot, "other_id.md"), []byte("# Lain\n\nkatakunci_beda\n"), 0o644)
	storage := t.TempDir()

	idx, err := NewIndex(docsRoot, storage, Options{SkipAutoBuild: true})
	if err != nil {
		t.Fatal(err)
	}
	gate, _ := idx.Gate(context.Background())
	if gate.Status != "missing" || gate.Usable {
		t.Fatalf("missing: %+v", gate)
	}
	any, err := idx.Search(context.Background(), "unique_word_xyz", 3)
	if err != nil {
		t.Fatal(err)
	}
	if hits, _ := any.([]Hit); len(hits) != 0 {
		t.Fatalf("not ready should empty: %#v", hits)
	}

	if err := idx.Reindex(context.Background()); err != nil {
		t.Fatal(err)
	}
	gate, _ = idx.Gate(context.Background())
	if !gate.Usable || gate.Status != "ready" {
		t.Fatalf("%+v", gate)
	}

	any, err = idx.Search(context.Background(), "unique_word_xyz", 0)
	if err != nil {
		t.Fatal(err)
	}
	hits, _ := any.([]Hit)
	if len(hits) == 0 {
		t.Fatal("expected hits")
	}

	// language filter
	en, err := idx.SearchHits(context.Background(), "unique_word_xyz", 5, Filters{Language: "en"})
	if err != nil || len(en) == 0 {
		t.Fatalf("en filter: %v %#v", err, en)
	}
	none, _ := idx.SearchHits(context.Background(), "unique_word_xyz", 5, Filters{Language: "xx"})
	if len(none) != 0 {
		t.Fatalf("xx filter: %#v", none)
	}

	// fuzzy / levenshtein path via near-miss token
	fuzzy, _ := idx.SearchHits(context.Background(), "unique_word_xyzz", 5, Filters{})
	if len(fuzzy) == 0 {
		t.Fatal("expected fuzzy hit")
	}

	// snippet truncate when query not in text
	noMatchSnippet, _ := idx.SearchHits(context.Background(), "zzzznotpresent999", 5, Filters{})
	_ = noMatchSnippet // may be empty; still exercises ranking empty query words

	// corrupt status → treat as ready if index exists
	_ = os.WriteFile(idx.statusPath(), []byte("{bad"), 0o644)
	if !idx.Usable() {
		t.Fatal("corrupt status with index should still usable via status()")
	}
	gate, _ = idx.Gate(context.Background())
	if gate.Status != "ready" {
		t.Fatalf("gate=%+v", gate)
	}

	// building / failed messages
	_ = idx.writeStatusLocked(statusMeta{Status: "building", DocumentCount: 1})
	gate, _ = idx.Gate(context.Background())
	if gate.Status != "building" || gate.Usable {
		t.Fatalf("%+v", gate)
	}
	_ = idx.markFailedLocked(strings.Repeat("e", 600))
	gate, _ = idx.Gate(context.Background())
	if gate.Status != "failed" || !strings.Contains(gate.Message, "e") {
		t.Fatalf("%+v", gate)
	}
	if len(gate.Message) > 500 {
		t.Fatalf("message not truncated: %d", len(gate.Message))
	}
}

func TestLoadCorruptAndEmptyIndex(t *testing.T) {
	docsRoot := t.TempDir()
	_ = os.WriteFile(filepath.Join(docsRoot, "a.md"), []byte("# A\n\nhello\n"), 0o644)
	storage := t.TempDir()
	idx, err := NewIndex(docsRoot, storage, Options{})
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(idx.indexPath(), []byte(""), 0o644)
	hits, err := idx.SearchHits(context.Background(), "hello", 5, Filters{})
	if err != nil || len(hits) != 0 {
		t.Fatalf("empty index: %v %#v", err, hits)
	}
	_ = os.WriteFile(idx.indexPath(), []byte("{notjson"), 0o644)
	// Usable still true (file exists + ready status) but load fails → empty hits
	hits, err = idx.SearchHits(context.Background(), "hello", 5, Filters{})
	if err != nil || len(hits) != 0 {
		t.Fatalf("corrupt index: %v %#v", err, hits)
	}
	_ = os.WriteFile(idx.indexPath(), []byte(`{"documents":null}`), 0o644)
	hits, _ = idx.SearchHits(context.Background(), "hello", 5, Filters{})
	if len(hits) != 0 {
		t.Fatalf("%#v", hits)
	}
}

func TestChunkAndLanguageHelpers(t *testing.T) {
	chunks := chunkMarkdown("# H1\n\npara one\n\n## H2\n\npara two with more text\n")
	if len(chunks) == 0 {
		t.Fatal("chunks")
	}
	if languageFromPath("writing/foo_en.md") != "en" {
		t.Fatal("lang en")
	}
	if languageFromPath("x_id.md") != "id" {
		t.Fatal("lang id")
	}
	if languageFromPath("plain.md") == "" {
		// may be empty or inferred — just call
	}
	if firstHeading("no heading\n", "fallback.md") == "" {
		t.Fatal("firstHeading fallback")
	}
	if !withinRoot("/a/b", "/a/b/c") || withinRoot("/a/b", "/a/bx") {
		t.Fatal("withinRoot")
	}
	if truncateRunes("abcdef", 3) != "abc" || truncateRunes("ab", 5) != "ab" {
		t.Fatal("truncateRunes")
	}
	if min(1, 2) != 1 || abs(-3) != 3 {
		t.Fatal("min/abs")
	}
	if snippet("hello world nowhere", "zzz missing", 10) == "" {
		t.Fatal("snippet truncate path")
	}
	s := snippet("prefix unique_token suffix more text here", "unique_token", 20)
	if !strings.Contains(s, "unique") {
		t.Fatalf("snippet=%q", s)
	}
}

func TestCollectDocumentsEmptyRoot(t *testing.T) {
	docs, err := collectDocuments(filepath.Join(t.TempDir(), "missing"), "app")
	if err != nil || len(docs) != 0 {
		t.Fatalf("%v %#v", err, docs)
	}
	// file as root → empty
	f := filepath.Join(t.TempDir(), "file")
	_ = os.WriteFile(f, []byte("x"), 0o644)
	docs, err = collectDocuments(f, "app")
	if err != nil || len(docs) != 0 {
		t.Fatalf("%v %#v", err, docs)
	}
}

func TestKeywordScoreAndDomainFilter(t *testing.T) {
	docsRoot := t.TempDir()
	_ = os.MkdirAll(filepath.Join(docsRoot, "writing"), 0o755)
	_ = os.WriteFile(filepath.Join(docsRoot, "writing", "guide_en.md"), []byte("# Guide\n\narchitecture pattern domain_token_abc\n"), 0o644)
	storage := t.TempDir()
	idx, err := NewIndex(docsRoot, storage, Options{DisableFuzzy: false, TopK: 3})
	if err != nil {
		t.Fatal(err)
	}
	hits, err := idx.SearchHits(context.Background(), "domain_token_abc architecture", 5, Filters{Domain: "writing"})
	if err != nil || len(hits) == 0 {
		t.Fatalf("%v %#v", err, hits)
	}
	none, _ := idx.SearchHits(context.Background(), "domain_token_abc", 5, Filters{Domain: "missing"})
	if len(none) != 0 {
		t.Fatalf("%#v", none)
	}
	empty, _ := idx.SearchHits(context.Background(), "   ", 5, Filters{})
	if len(empty) != 0 {
		t.Fatalf("%#v", empty)
	}
	// DisableFuzzy path
	idx2, err := NewIndex(docsRoot, t.TempDir(), Options{DisableFuzzy: true})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = idx2.SearchHits(context.Background(), "domain_token_abc", 3, Filters{})
}

func TestHeadingLineAndChunkEdge(t *testing.T) {
	if headingLine("# Title") == "" || headingLine("not") != "" {
		t.Fatal("headingLine")
	}
	if headingLine("#### too deep") != "" || headingLine("#nospace") != "" {
		t.Fatal("heading edges")
	}
	chunks := chunkMarkdown("")
	if len(chunks) != 1 {
		t.Fatalf("empty text chunks=%d", len(chunks))
	}
	chunks = chunkMarkdown("# Only\n")
	if len(chunks) == 0 {
		t.Fatal("expected chunk")
	}
}

