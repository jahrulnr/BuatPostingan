package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// docsFilesystem sandboxes list_dir / read_file / grep under docsRoot.
type docsFilesystem struct {
	root string
}

func newDocsFilesystem(docsRoot string) (*docsFilesystem, error) {
	abs, err := filepath.Abs(docsRoot)
	if err != nil {
		return nil, fmt.Errorf("docs root unavailable: %w", err)
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if st, stErr := os.Stat(abs); stErr != nil || !st.IsDir() {
			return nil, fmt.Errorf("docs root unavailable")
		}
		real = abs
	}
	st, err := os.Stat(real)
	if err != nil || !st.IsDir() {
		return nil, fmt.Errorf("docs root unavailable")
	}
	return &docsFilesystem{root: real}, nil
}

func (fs *docsFilesystem) listDir(args map[string]any) (map[string]any, error) {
	relative, err := fs.relativePath(asString(args["path"]))
	if err != nil {
		return nil, err
	}
	directory, err := fs.resolve(relative, true)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(directory)
	if err != nil || !st.IsDir() {
		return failMap("not_directory", "path is not a directory"), nil
	}

	limit := clamp(asInt(args["max_entries"], 50), 1, 100)
	offset := clamp(asInt(args["offset"], 0), 0, 100000)

	entriesDir, err := os.ReadDir(directory)
	if err != nil {
		return failMap("tool_error", "directory could not be read"), nil
	}
	var entries []map[string]any
	for _, e := range entriesDir {
		name := e.Name()
		full := filepath.Join(directory, name)
		info, infoErr := e.Info()
		typ := "file"
		if e.IsDir() {
			typ = "directory"
		}
		entry := map[string]any{
			"name": name,
			"path": fs.relativeToRoot(full),
			"type": typ,
		}
		if infoErr == nil && info != nil {
			entry["mode"] = info.Mode().String()
			entry["size"] = info.Size()
			entry["mtime"] = info.ModTime().UTC().Format(time.RFC3339)
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		ti, _ := entries[i]["type"].(string)
		tj, _ := entries[j]["type"].(string)
		if ti != tj {
			return ti < tj // directory before file? PHP uses [type, name] — "directory" < "file"
		}
		ni, _ := entries[i]["name"].(string)
		nj, _ := entries[j]["name"].(string)
		return ni < nj
	})
	total := len(entries)
	if offset > total {
		offset = total
	}
	end := min(offset + limit, total)
	page := entries[offset:end]
	nextOffset := offset + len(page)
	hasMore := nextOffset < total
	var next any
	if hasMore {
		next = nextOffset
	} else {
		next = nil
	}
	// Always include an ls -lah style listing (with . and ..) so empty dirs
	// are never a barren entries:[] with no readable context for the model/UI.
	listing := formatLSListing(directory, st, page, total)
	return okMap(map[string]any{
		"path":        relative,
		"entries":     page,
		"listing":     listing,
		"offset":      offset,
		"has_more":    hasMore,
		"next_offset": next,
		"total":       total,
	}, hasMore), nil
}

// formatLSListing builds an ls -lah–like text block. Even when page is empty,
// it still shows total + . + .. so the model sees a real directory listing.
func formatLSListing(directory string, dirInfo os.FileInfo, page []map[string]any, total int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "total %d\n", total)

	writeLine := func(mode string, size int64, mtime time.Time, name string) {
		if mode == "" {
			mode = "----------"
		}
		ts := mtime.UTC().Format("Jan 02 15:04")
		fmt.Fprintf(&b, "%-10s %8d %s %s\n", mode, size, ts, name)
	}

	if dirInfo != nil {
		writeLine(dirInfo.Mode().String(), dirInfo.Size(), dirInfo.ModTime(), ".")
	} else {
		writeLine("d---------", 0, time.Time{}, ".")
	}
	parent := filepath.Dir(directory)
	if pst, err := os.Stat(parent); err == nil {
		writeLine(pst.Mode().String(), pst.Size(), pst.ModTime(), "..")
	} else {
		writeLine("d---------", 0, time.Time{}, "..")
	}

	for _, e := range page {
		name, _ := e["name"].(string)
		typ, _ := e["type"].(string)
		mode, _ := e["mode"].(string)
		var size int64
		switch s := e["size"].(type) {
		case int64:
			size = s
		case int:
			size = int64(s)
		case float64:
			size = int64(s)
		}
		mtime := time.Time{}
		if ms, ok := e["mtime"].(string); ok {
			if t, err := time.Parse(time.RFC3339, ms); err == nil {
				mtime = t
			}
		}
		display := name
		if typ == "directory" {
			display = name + "/"
		}
		writeLine(mode, size, mtime, display)
	}
	return strings.TrimRight(b.String(), "\n")
}

func (fs *docsFilesystem) readFile(args map[string]any) (map[string]any, error) {
	relative, err := fs.relativePath(asString(args["path"]))
	if err != nil {
		return nil, err
	}
	file, err := fs.resolve(relative, false)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(file)
	if err != nil || st.IsDir() {
		return failMap("not_file", "path is not a file"), nil
	}
	if strings.ToLower(filepath.Ext(file)) != ".md" {
		return failMap("file_type_not_allowed", "only Markdown files are readable"), nil
	}

	maxChars := clamp(asInt(args["max_chars"], 12000), 1, 20000)
	offset := clamp(asInt(args["offset"], 0), 0, 1000000)
	contentBytes, err := os.ReadFile(file)
	if err != nil {
		return failMap("read_failed", "file could not be read"), nil
	}
	runes := []rune(string(contentBytes))
	length := len(runes)
	if offset > length {
		offset = length
	}
	end := offset + maxChars
	if end > length {
		end = length
	}
	slice := string(runes[offset:end])
	nextOffset := offset + utf8.RuneCountInString(slice)
	hasMore := nextOffset < length
	var next any
	if hasMore {
		next = nextOffset
	} else {
		next = nil
	}
	return okMap(map[string]any{
		"path":        relative,
		"content":     slice,
		"offset":      offset,
		"has_more":    hasMore,
		"next_offset": next,
		"total_chars": length,
		"truncated":   hasMore,
	}, hasMore), nil
}

func (fs *docsFilesystem) relativePath(value string) (string, error) {
	path := strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
	if path == "" || path == "." {
		return "", nil
	}
	if strings.HasPrefix(path, "/") || strings.Contains(path, "\x00") {
		return "", fmt.Errorf("path traversal is not allowed")
	}
	for _, p := range strings.Split(path, "/") {
		if p == "." || p == ".." {
			return "", fmt.Errorf("path traversal is not allowed")
		}
	}
	return strings.Trim(path, "/"), nil
}

func (fs *docsFilesystem) resolve(relative string, directoryAllowed bool) (string, error) {
	candidate := fs.root
	if relative != "" {
		candidate = filepath.Join(fs.root, filepath.FromSlash(relative))
	}
	real, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		// Non-existent path: still check lexical containment then fail outside.
		clean := filepath.Clean(candidate)
		if !withinRoot(fs.root, clean) {
			return "", fmt.Errorf("path is outside docs root")
		}
		return "", fmt.Errorf("path is outside docs root")
	}
	if !withinRoot(fs.root, real) {
		return "", fmt.Errorf("path is outside docs root")
	}
	if !directoryAllowed {
		st, err := os.Stat(real)
		if err != nil || st.IsDir() {
			return "", fmt.Errorf("file path required")
		}
	}
	return real, nil
}

func (fs *docsFilesystem) markdownFiles(directory string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(directory, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(d.Name()), ".md") {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

func (fs *docsFilesystem) relativeToRoot(path string) string {
	rel, err := filepath.Rel(fs.root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
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

func okMap(data map[string]any, truncated bool) map[string]any {
	count := 0
	if c, ok := data["count"].(int); ok {
		count = c
	} else if entries, ok := data["entries"].([]map[string]any); ok {
		count = len(entries)
	} else if matches, ok := data["matches"].([]map[string]any); ok {
		count = len(matches)
	}
	return map[string]any{
		"ok":   true,
		"tool": "docs_filesystem",
		"data": data,
		"error": nil,
		"meta": map[string]any{
			"truncated":         truncated,
			"count":             count,
			"data_is_untrusted": true,
		},
	}
}

func failMap(code, message string) map[string]any {
	return map[string]any{
		"ok":   false,
		"tool": "docs_filesystem",
		"data": nil,
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
		"meta": map[string]any{
			"truncated":         false,
			"count":             0,
			"data_is_untrusted": true,
		},
	}
}
