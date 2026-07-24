package docs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func (idx *Index) buildLocked() (map[string]any, error) {
	documents, err := collectDocuments(idx.docsRoot, idx.opts.AppID)
	if err != nil {
		return nil, err
	}
	payload := indexFile{
		BuiltAt:       float64(time.Now().UnixNano()) / 1e9,
		DocsRoot:      idx.docsRoot,
		AppID:         idx.opts.AppID,
		DocumentCount: len(documents),
		Documents:     documents,
	}

	if err := idx.ensureStorageDir(); err != nil {
		return nil, err
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("docs_index encode failed: %w", err)
	}

	path := idx.indexPath()
	tmp := fmt.Sprintf("%s.%d.tmp", path, os.Getpid())
	if err := os.WriteFile(tmp, raw, 0o664); err != nil {
		return nil, fmt.Errorf("docs_index write failed: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return nil, fmt.Errorf("docs_index rename failed: %w", err)
	}

	if err := idx.writeStatusLocked(statusMeta{
		Status:        "ready",
		At:            float64(time.Now().UnixNano()) / 1e9,
		DocumentCount: len(documents),
	}); err != nil {
		return nil, err
	}

	return map[string]any{
		"document_count": len(documents),
		"path":           path,
	}, nil
}

func collectDocuments(docsRoot, appID string) ([]document, error) {
	info, err := os.Stat(docsRoot)
	if err != nil || !info.IsDir() {
		return []document{}, nil
	}
	rootReal, err := filepath.Abs(docsRoot)
	if err != nil {
		return []document{}, nil
	}
	rootReal, err = filepath.EvalSymlinks(rootReal)
	if err != nil {
		rootReal, _ = filepath.Abs(docsRoot)
	}

	var documents []document
	err = filepath.WalkDir(rootReal, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}
		real, err := filepath.EvalSymlinks(path)
		if err != nil {
			real = path
		}
		if !withinRoot(rootReal, real) {
			return nil
		}
		textBytes, err := os.ReadFile(real)
		if err != nil {
			return nil
		}
		text := string(textBytes)
		rel, err := filepath.Rel(rootReal, real)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		fi, _ := d.Info()
		var mtime int64
		if fi != nil {
			mtime = fi.ModTime().Unix()
		}
		documents = append(documents, document{
			Path:     rel,
			Title:    firstHeading(text, rel),
			Headings: extractHeadings(text),
			Text:     text,
			Language: languageFromPath(rel),
			Domain:   domainFromPath(rel),
			AppID:    appID,
			Chunks:   chunkMarkdown(text),
			MTime:    mtime,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(documents, func(i, j int) bool {
		return documents[i].Path < documents[j].Path
	})
	return documents, nil
}

func withinRoot(root, path string) bool {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	if path == root {
		return true
	}
	sep := string(filepath.Separator)
	return strings.HasPrefix(path, root+sep)
}

func chunkMarkdown(text string) []chunk {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var chunks []chunk
	heading := ""
	var body []string
	flush := func() {
		content := strings.TrimSpace(strings.Join(body, "\n"))
		if content != "" {
			chunks = append(chunks, chunk{
				ID:      fmt.Sprintf("chunk_%d", len(chunks)),
				Heading: heading,
				Text:    content,
			})
		}
		body = nil
	}
	for _, line := range lines {
		if m := headingLine(line); m != "" {
			flush()
			heading = m
			continue
		}
		body = append(body, line)
	}
	flush()
	if len(chunks) == 0 {
		return []chunk{{ID: "chunk_0", Heading: "", Text: strings.TrimSpace(text)}}
	}
	return chunks
}

func headingLine(line string) string {
	trimmed := strings.TrimRight(line, " \t")
	n := 0
	for n < len(trimmed) && trimmed[n] == '#' {
		n++
	}
	if n < 1 || n > 3 {
		return ""
	}
	if n >= len(trimmed) || (trimmed[n] != ' ' && trimmed[n] != '\t') {
		return ""
	}
	return strings.TrimSpace(trimmed[n+1:])
}

func languageFromPath(path string) string {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, "_id.md") {
		return "id"
	}
	if strings.HasSuffix(lower, "_en.md") {
		return "en"
	}
	return "unknown"
}

func domainFromPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) > 1 {
		return parts[0]
	}
	return "general"
}

func extractHeadings(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		n := 0
		for n < len(line) && line[n] == '#' {
			n++
		}
		if n == 0 || n >= len(line) || (line[n] != ' ' && line[n] != '\t') {
			continue
		}
		out = append(out, strings.TrimSpace(line[n+1:]))
	}
	return out
}

func firstHeading(text, path string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(line[2:])
		}
	}
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}
