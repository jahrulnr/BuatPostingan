package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// workspaceFS resolves list_dir / read_file / grep against the real filesystem.
// Empty base means relative paths resolve via filepath.Abs (process cwd); absolute
// paths are always unrestricted (including "/"). A non-empty base (e.g. test temp
// dir) is only the default for relative paths — not a jail.
type workspaceFS struct {
	base string
}

var embeddedToolParameterRe = regexp.MustCompile(`(?s)<parameter=\w+>.*?</parameter>`)

func stripEmbeddedToolParameters(content string) string {
	return embeddedToolParameterRe.ReplaceAllString(content, "")
}

func newWorkspaceFS(fsRoot string) (*workspaceFS, error) {
	fsRoot = strings.TrimSpace(fsRoot)
	if fsRoot == "" {
		return &workspaceFS{base: ""}, nil
	}
	abs, err := filepath.Abs(fsRoot)
	if err != nil {
		return nil, fmt.Errorf("fs root unavailable: %w", err)
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if st, stErr := os.Stat(abs); stErr != nil || !st.IsDir() {
			return nil, fmt.Errorf("fs root unavailable")
		}
		real = abs
	}
	st, err := os.Stat(real)
	if err != nil || !st.IsDir() {
		return nil, fmt.Errorf("fs root unavailable")
	}
	return &workspaceFS{base: real}, nil
}

func (fs *workspaceFS) listDir(args map[string]any) (map[string]any, error) {
	requested := asString(args["path"])
	directory, err := fs.resolvePath(requested)
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
			"path": fs.displayPath(full),
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
			return ti < tj
		}
		ni, _ := entries[i]["name"].(string)
		nj, _ := entries[j]["name"].(string)
		return ni < nj
	})
	total := len(entries)
	if offset > total {
		offset = total
	}
	end := min(offset+limit, total)
	page := entries[offset:end]
	nextOffset := offset + len(page)
	hasMore := nextOffset < total
	var next any
	if hasMore {
		next = nextOffset
	} else {
		next = nil
	}
	listing := formatLSListing(directory, st, page, total)
	return okMap(map[string]any{
		"path":        fs.displayPath(directory),
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

func (fs *workspaceFS) readFile(args map[string]any) (map[string]any, error) {
	file, err := fs.resolvePath(asString(args["path"]))
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(file)
	if err != nil || st.IsDir() {
		return failMap("not_file", "path is not a file"), nil
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
		"path":        fs.displayPath(file),
		"content":     slice,
		"offset":      offset,
		"has_more":    hasMore,
		"next_offset": next,
		"total_chars": length,
		"truncated":   hasMore,
	}, hasMore), nil
}

func (fs *workspaceFS) writeFile(args map[string]any) (map[string]any, error) {
	if strings.TrimSpace(asString(args["path"])) == "" {
		return failMap("validation", "path is required"), nil
	}
	if _, ok := args["content"]; !ok {
		return failMap("validation", "content is required"), nil
	}

	file, err := fs.resolvePath(asString(args["path"]))
	if err != nil {
		return nil, err
	}
	if info, err := os.Stat(file); err == nil && info.IsDir() {
		return failMap("path_is_directory", "path is a directory"), nil
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return failMap("mkdir_failed", fmt.Sprintf("could not create parent directory: %v", err)), nil
	}

	appendMode := asBool(args["append"], false)
	flag := os.O_CREATE | os.O_WRONLY
	if appendMode {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_TRUNC
	}
	f, err := os.OpenFile(file, flag, 0o644)
	if err != nil {
		return failMap("write_failed", fmt.Sprintf("could not open file for writing: %v", err)), nil
	}
	content := stripEmbeddedToolParameters(asString(args["content"]))
	_, werr := f.WriteString(content)
	if cerr := f.Close(); cerr != nil && werr == nil {
		werr = cerr
	}
	if werr != nil {
		return failMap("write_failed", fmt.Sprintf("could not write file: %v", werr)), nil
	}
	return okMap(map[string]any{
		"path":          fs.displayPath(file),
		"written_bytes": len(content),
	}, false), nil
}

func (fs *workspaceFS) editFile(args map[string]any) (map[string]any, error) {
	if strings.TrimSpace(asString(args["path"])) == "" {
		return failMap("validation", "path is required"), nil
	}
	if _, ok := args["old_string"]; !ok {
		return failMap("validation", "old_string is required"), nil
	}
	if _, ok := args["new_string"]; !ok {
		return failMap("validation", "new_string is required"), nil
	}

	file, err := fs.resolvePath(asString(args["path"]))
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(file)
	if err != nil || st.IsDir() {
		return failMap("not_file", "path is not a file"), nil
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return failMap("read_failed", fmt.Sprintf("file could not be read: %v", err)), nil
	}

	old := asString(args["old_string"])
	newStr := asString(args["new_string"])
	if old == "" {
		return failMap("empty_old_string", "old_string cannot be empty"), nil
	}

	content := string(b)
	if !strings.Contains(content, old) {
		return failMap("old_string_not_found", "old_string not found in file"), nil
	}

	replaceAll := asBool(args["replace_all"], false)
	var edited string
	var count int
	if replaceAll {
		edited = strings.ReplaceAll(content, old, newStr)
		count = strings.Count(content, old)
	} else {
		edited = strings.Replace(content, old, newStr, 1)
		count = 1
	}

	if err := os.WriteFile(file, []byte(edited), st.Mode()); err != nil {
		return failMap("write_failed", fmt.Sprintf("could not write edited file: %v", err)), nil
	}
	return okMap(map[string]any{
		"path":         fs.displayPath(file),
		"replacements": count,
	}, false), nil
}

func (fs *workspaceFS) deleteFile(args map[string]any) (map[string]any, error) {
	if strings.TrimSpace(asString(args["path"])) == "" {
		return failMap("validation", "path is required"), nil
	}

	file, err := fs.resolvePath(asString(args["path"]))
	if err != nil {
		return nil, err
	}
	recursive := asBool(args["recursive"], false)
	st, err := os.Stat(file)
	if err != nil {
		return failMap("not_found", fmt.Sprintf("path does not exist: %v", err)), nil
	}

	if st.IsDir() {
		if recursive {
			err = os.RemoveAll(file)
		} else {
			err = os.Remove(file)
		}
	} else {
		err = os.Remove(file)
	}
	if err != nil {
		return failMap("delete_failed", fmt.Sprintf("could not delete path: %v", err)), nil
	}
	return okMap(map[string]any{
		"path":    fs.displayPath(file),
		"deleted": true,
	}, false), nil
}

// resolvePath maps a tool path argument to an absolute filesystem path.
// Absolute paths (including "/") are accepted as-is. Relative paths join the
// optional base, or resolve via filepath.Abs when base is empty. Symlinks are
// followed; there is no workspace jail.
func (fs *workspaceFS) resolvePath(value string) (string, error) {
	path := strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
	if strings.Contains(path, "\x00") {
		return "", fmt.Errorf("invalid path")
	}
	if path == "" || path == "." {
		if fs.base != "" {
			return fs.base, nil
		}
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("cwd unavailable: %w", err)
		}
		return cwd, nil
	}

	native := filepath.FromSlash(path)
	var candidate string
	if filepath.IsAbs(native) {
		candidate = filepath.Clean(native)
	} else if fs.base != "" {
		candidate = filepath.Join(fs.base, native)
	} else {
		abs, err := filepath.Abs(native)
		if err != nil {
			return "", err
		}
		candidate = abs
	}

	real, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return filepath.Clean(candidate), nil
	}
	return real, nil
}

func (fs *workspaceFS) listFiles(directory string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(directory, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	sort.Strings(files)
	return files, err
}

func (fs *workspaceFS) displayPath(path string) string {
	if fs.base != "" {
		if rel, err := filepath.Rel(fs.base, path); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(path)
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
		"ok":    true,
		"tool":  "workspace_fs",
		"data":  data,
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
		"tool": "workspace_fs",
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
