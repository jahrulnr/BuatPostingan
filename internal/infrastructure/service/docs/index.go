package docs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/service"
)

var _ service.DocsIndex = (*Index)(nil)

// Index is a lexical Markdown docs RAG index persisted under storageRoot.
type Index struct {
	docsRoot    string
	storageRoot string
	opts        Options

	mu sync.RWMutex
}

// NewIndex creates an Index. When SkipAutoBuild is false and docs_index.json
// is missing, it builds synchronously once (v0). Wire agents may still call
// Reindex at startup to refresh.
func NewIndex(docsRoot, storageRoot string, opts Options) (*Index, error) {
	if docsRoot == "" {
		return nil, fmt.Errorf("docs: docsRoot required")
	}
	if storageRoot == "" {
		return nil, fmt.Errorf("docs: storageRoot required")
	}
	idx := &Index{
		docsRoot:    docsRoot,
		storageRoot: storageRoot,
		opts:        opts.withDefaults(),
	}
	if !idx.opts.SkipAutoBuild && !idx.indexFileExists() {
		if err := idx.Reindex(context.Background()); err != nil {
			return nil, fmt.Errorf("docs: auto-build: %w", err)
		}
	}
	return idx, nil
}

func (idx *Index) indexPath() string {
	return filepath.Join(idx.storageRoot, "docs_index.json")
}

func (idx *Index) statusPath() string {
	return filepath.Join(idx.storageRoot, "docs_index.status.json")
}

func (idx *Index) indexFileExists() bool {
	_, err := os.Stat(idx.indexPath())
	return err == nil
}

// Usable reports whether AI turns may run (status=ready AND index file exists).
func (idx *Index) Usable() bool {
	return idx.status() == "ready" && idx.indexFileExists()
}

// Gate implements service.DocsIndex.
func (idx *Index) Gate(ctx context.Context) (entity.DocsIndexGate, error) {
	_ = ctx
	meta := idx.statusMeta()
	st := idx.status()
	usable := st == "ready" && idx.indexFileExists()
	msg := "Docs index siap."
	switch st {
	case "building":
		msg = "Docs index sedang dibangun. AI sementara tidak tersedia."
	case "failed":
		if meta.Message != "" {
			msg = meta.Message
		} else {
			msg = "Docs index gagal dibangun. Restart untuk coba lagi."
		}
	case "missing":
		msg = "Docs index belum siap."
	}
	return entity.DocsIndexGate{
		Usable:        usable,
		Status:        st,
		Message:       msg,
		DocumentCount: meta.DocumentCount,
	}, nil
}

// Search implements service.DocsIndex (no language/domain filters).
func (idx *Index) Search(ctx context.Context, query string, topK int) (any, error) {
	hits, err := idx.SearchHits(ctx, query, topK, Filters{})
	if err != nil {
		return nil, err
	}
	return hits, nil
}

// SearchHits ranks chunks; returns empty when index is not usable.
func (idx *Index) SearchHits(ctx context.Context, query string, topK int, filters Filters) ([]Hit, error) {
	_ = ctx
	if !idx.Usable() {
		return []Hit{}, nil
	}
	data, err := idx.load()
	if err != nil || data == nil {
		return []Hit{}, nil
	}
	if topK <= 0 {
		topK = idx.opts.TopK
	}
	return rankDocuments(data.Documents, query, topK, filters, idx.opts), nil
}

// Reindex rebuilds the index synchronously (v0). Marks building, then ready/failed.
func (idx *Index) Reindex(ctx context.Context) error {
	_ = ctx
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if err := idx.beginReindexLocked(); err != nil {
		return err
	}
	if _, err := idx.buildLocked(); err != nil {
		_ = idx.markFailedLocked(err.Error())
		return err
	}
	return nil
}

func (idx *Index) status() string {
	meta := idx.statusMeta()
	st := meta.Status
	if st == "building" || st == "failed" {
		return st
	}
	if st == "ready" && idx.indexFileExists() {
		return "ready"
	}
	if idx.indexFileExists() {
		return "ready"
	}
	return "missing"
}

func (idx *Index) statusMeta() statusMeta {
	path := idx.statusPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		if idx.indexFileExists() {
			return statusMeta{Status: "ready"}
		}
		return statusMeta{Status: "missing"}
	}
	var meta statusMeta
	if err := json.Unmarshal(raw, &meta); err != nil || meta.Status == "" {
		if idx.indexFileExists() {
			return statusMeta{Status: "ready"}
		}
		return statusMeta{Status: "missing"}
	}
	return meta
}

func (idx *Index) ensureStorageDir() error {
	return os.MkdirAll(idx.storageRoot, 0o775)
}

func (idx *Index) writeStatusLocked(meta statusMeta) error {
	if err := idx.ensureStorageDir(); err != nil {
		return err
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		raw = []byte(`{"status":"failed"}`)
	}
	return os.WriteFile(idx.statusPath(), raw, 0o664)
}

func (idx *Index) beginReindexLocked() error {
	if err := idx.ensureStorageDir(); err != nil {
		return err
	}
	_ = os.Remove(idx.indexPath())
	return idx.writeStatusLocked(statusMeta{
		Status: "building",
		At:     float64(time.Now().UnixNano()) / 1e9,
	})
}

func (idx *Index) markFailedLocked(message string) error {
	if len(message) > 500 {
		message = message[:500]
	}
	return idx.writeStatusLocked(statusMeta{
		Status:  "failed",
		At:      float64(time.Now().UnixNano()) / 1e9,
		Message: message,
	})
}

func (idx *Index) load() (*indexFile, error) {
	if !idx.Usable() {
		return nil, nil
	}
	raw, err := os.ReadFile(idx.indexPath())
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, nil
	}
	var data indexFile
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	if data.Documents == nil {
		return nil, nil
	}
	return &data, nil
}
