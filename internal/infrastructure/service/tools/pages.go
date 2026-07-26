package tools

import (
	"bufio"
	"errors"
	"fmt"
	"html"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"buatpostingan/internal/domain/service"
)

const (
	publishedDirName  = ".published"
	maxPageSearchSize = 1 << 20
	maxPageEditBytes  = 1 << 20
)

var (
	pageIDPattern      = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,78}[a-z0-9])?$`)
	errPublishedRoot   = errors.New("published directory must be a real directory")
	errInvalidPagePath = errors.New("path must be a relative text file inside the page workspace")
	pageTextExtensions = map[string]bool{
		".css": true, ".htm": true, ".html": true, ".js": true, ".json": true,
		".md": true, ".mjs": true, ".svg": true, ".txt": true, ".xml": true,
	}
)

// pagesFS owns only the page workspace. Page tools accept a slug, never a path,
// so publication cannot be redirected outside this root.
type pagesFS struct{ root string }

func newPagesFS(root string) (*pagesFS, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return &pagesFS{}, nil
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("pages root unavailable: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("pages root unavailable: %w", err)
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, fmt.Errorf("pages root unavailable: %w", err)
	}
	st, err := os.Stat(real)
	if err != nil || !st.IsDir() {
		return nil, fmt.Errorf("pages root unavailable")
	}
	return &pagesFS{root: real}, nil
}

func (r *Registry) execPages(name string, args map[string]any) service.ToolEnvelope {
	if r.pages == nil || r.pages.root == "" {
		return pageFail(name, "pages_unavailable", "page workspace is not configured")
	}
	switch name {
	case "page_list":
		return r.pages.list()
	case "page_search":
		return r.pages.search(args)
	case "page_create":
		return r.pages.create(asString(args["page_id"]), asString(args["title"]))
	case "page_edit":
		return r.pages.edit(asString(args["page_id"]), args)
	case "page_read":
		return r.pages.read(asString(args["page_id"]), args)
	case "page_publish":
		return r.pages.publish(asString(args["page_id"]))
	case "page_unpublish":
		return r.pages.unpublish(asString(args["page_id"]))
	default:
		return pageFail(name, "unknown_tool", "unknown page tool")
	}
}

func (p *pagesFS) create(rawID, rawTitle string) service.ToolEnvelope {
	id, env := validatePageID("page_create", rawID)
	if env != nil {
		return *env
	}
	title := strings.TrimSpace(rawTitle)
	if title == "" {
		title = id
	}
	if utf8.RuneCountInString(title) > 200 {
		return pageFail("page_create", "validation", "title must not exceed 200 characters")
	}
	root := filepath.Join(p.root, id)
	if _, err := os.Lstat(root); err == nil {
		return pageFail("page_create", "conflict", "page workspace already exists")
	} else if !os.IsNotExist(err) {
		return pageFail("page_create", "tool_error", "page workspace could not be inspected")
	}
	if err := os.Mkdir(root, 0o755); err != nil {
		return pageFail("page_create", "tool_error", "page workspace could not be created")
	}
	if err := os.Mkdir(filepath.Join(root, "assets"), 0o755); err != nil {
		return pageFail("page_create", "tool_error", "page assets directory could not be created")
	}
	starter := map[string]string{
		"index.html": "<!doctype html>\n<html lang=\"id\">\n<head>\n  <meta charset=\"utf-8\">\n  <meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n  <title>" + html.EscapeString(title) + "</title>\n  <link rel=\"stylesheet\" href=\"page.css\">\n</head>\n<body>\n  <main>\n    <h1>" + html.EscapeString(title) + "</h1>\n  </main>\n  <script type=\"module\" src=\"page.js\"></script>\n</body>\n</html>\n",
		"page.css":   "/* Page styles */\n",
		"page.js":    "// Page behavior\n",
	}
	for name, content := range starter {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			return pageFail("page_create", "tool_error", "starter files could not be written")
		}
	}
	return pageOK("page_create", map[string]any{"page": p.pageSummary(id), "created": true}, 1)
}

func (p *pagesFS) edit(rawID string, args map[string]any) service.ToolEnvelope {
	id, env := validatePageID("page_edit", rawID)
	if env != nil {
		return *env
	}
	if _, ok := args["content"]; !ok {
		return pageFail("page_edit", "validation", "content is required")
	}
	mode := strings.ToLower(strings.TrimSpace(asString(args["mode"])))
	if mode == "" {
		mode = "overwrite"
	}
	if mode != "overwrite" && mode != "append" && mode != "replace" {
		return pageFail("page_edit", "validation", "mode must be overwrite, append, or replace")
	}
	old := asString(args["old_string"])
	if mode == "replace" && old == "" {
		return pageFail("page_edit", "validation", "old_string is required for replace mode")
	}
	content := stripEmbeddedToolParameters(asString(args["content"]))
	if len([]byte(content)) > maxPageEditBytes {
		return pageFail("page_edit", "validation", "content exceeds the 1 MiB page text limit")
	}
	file, err := p.pageFile(id, asString(args["path"]), true)
	if err != nil {
		return pagePathFail("page_edit", err)
	}
	if mode == "replace" {
		current, err := os.ReadFile(file)
		if os.IsNotExist(err) {
			return pageFail("page_edit", "not_found", "page file does not exist")
		}
		if err != nil {
			return pageFail("page_edit", "tool_error", "page file could not be read")
		}
		if !strings.Contains(string(current), old) {
			return pageFail("page_edit", "old_string_not_found", "old_string not found in page file")
		}
		updated := strings.Replace(string(current), old, content, 1)
		if err := os.WriteFile(file, []byte(updated), 0o644); err != nil {
			return pageFail("page_edit", "tool_error", "page file could not be written")
		}
		return pageOK("page_edit", map[string]any{"page_id": id, "path": file, "mode": mode, "replacements": 1}, 1)
	}
	flags := os.O_CREATE | os.O_WRONLY
	if mode == "append" {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(file, flags, 0o644)
	if err != nil {
		return pageFail("page_edit", "tool_error", "page file could not be opened")
	}
	_, writeErr := f.WriteString(content)
	if closeErr := f.Close(); writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		return pageFail("page_edit", "tool_error", "page file could not be written")
	}
	return pageOK("page_edit", map[string]any{"page_id": id, "path": file, "mode": mode, "written_bytes": len(content)}, 1)
}

func (p *pagesFS) read(rawID string, args map[string]any) service.ToolEnvelope {
	id, env := validatePageID("page_read", rawID)
	if env != nil {
		return *env
	}
	file, err := p.pageFile(id, asString(args["path"]), false)
	if err != nil {
		return pagePathFail("page_read", err)
	}
	if info, err := os.Stat(file); err == nil && info.Size() > maxPageEditBytes {
		return pageFail("page_read", "validation", "page file exceeds the 1 MiB text limit")
	}
	b, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		return pageFail("page_read", "not_found", "page file does not exist")
	}
	if err != nil {
		return pageFail("page_read", "tool_error", "page file could not be read")
	}
	runes := []rune(string(b))
	offset := clamp(asInt(args["offset"], 0), 0, len(runes))
	maxChars := clamp(asInt(args["max_chars"], 12000), 1, 20000)
	end := min(offset+maxChars, len(runes))
	content := string(runes[offset:end])
	hasMore := end < len(runes)
	var next any
	if hasMore {
		next = offset + utf8.RuneCountInString(content)
	}
	return pageOK("page_read", map[string]any{"page_id": id, "path": file, "content": content, "offset": offset, "has_more": hasMore, "next_offset": next, "total_chars": len(runes)}, 1)
}

func (p *pagesFS) list() service.ToolEnvelope {
	entries, err := os.ReadDir(p.root)
	if err != nil {
		return pageFail("page_list", "tool_error", "page workspace could not be read")
	}
	pages := make([]map[string]any, 0)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") || !entry.IsDir() || !pageIDPattern.MatchString(entry.Name()) {
			continue
		}
		info, err := os.Lstat(filepath.Join(p.root, entry.Name()))
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		pages = append(pages, p.pageSummary(entry.Name()))
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i]["id"].(string) < pages[j]["id"].(string) })
	return pageOK("page_list", map[string]any{"pages_root": p.root, "pages": pages}, len(pages))
}

func (p *pagesFS) search(args map[string]any) service.ToolEnvelope {
	query := strings.TrimSpace(asString(args["query"]))
	if query == "" {
		return pageFail("page_search", "validation", "query is required")
	}
	filterID := strings.TrimSpace(asString(args["page_id"]))
	if filterID != "" && !pageIDPattern.MatchString(filterID) {
		return pageFail("page_search", "validation", "page_id must be a lowercase slug")
	}
	maxResults := clamp(asInt(args["max_results"], 30), 1, 100)
	needle := strings.ToLower(query)
	matches := make([]map[string]any, 0)
	pages, err := p.pageIDs(filterID)
	if err != nil {
		return pageFail("page_search", "tool_error", "page workspace could not be read")
	}
	for _, id := range pages {
		if len(matches) >= maxResults {
			break
		}
		root := filepath.Join(p.root, id)
		_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() || len(matches) >= maxResults || !pageTextExtensions[strings.ToLower(filepath.Ext(entry.Name()))] {
				return nil
			}
			info, err := entry.Info()
			if err != nil || info.Size() > maxPageSearchSize || info.Mode()&os.ModeSymlink != 0 {
				return nil
			}
			file, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer file.Close()
			scanner := bufio.NewScanner(file)
			scanner.Buffer(make([]byte, 4096), 256*1024)
			for line := 1; scanner.Scan() && len(matches) < maxResults; line++ {
				text := scanner.Text()
				if strings.Contains(strings.ToLower(text), needle) {
					matches = append(matches, map[string]any{"page_id": id, "path": path, "line": line, "excerpt": truncatePageExcerpt(strings.TrimSpace(text)), "published": p.isPublished(id)})
				}
			}
			return nil
		})
	}
	return pageOK("page_search", map[string]any{"query": query, "matches": matches}, len(matches))
}

func (p *pagesFS) publish(rawID string) service.ToolEnvelope {
	id, env := validatePageID("page_publish", rawID)
	if env != nil {
		return *env
	}
	pagePath := filepath.Join(p.root, id)
	info, err := os.Lstat(pagePath)
	if os.IsNotExist(err) || (err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0)) {
		return pageFail("page_publish", "not_found", "page workspace does not exist")
	}
	if err != nil {
		return pageFail("page_publish", "tool_error", "page workspace could not be read")
	}
	publishedRoot, err := p.publishedRoot(true)
	if err != nil {
		if errors.Is(err, errPublishedRoot) {
			return pageFail("page_publish", "conflict", err.Error())
		}
		return pageFail("page_publish", "tool_error", "published directory could not be created")
	}
	link := filepath.Join(publishedRoot, id)
	if info, err := os.Lstat(link); err == nil {
		if info.Mode()&os.ModeSymlink == 0 || !p.isPublished(id) {
			return pageFail("page_publish", "conflict", "published marker exists but does not point to this page")
		}
		return pageOK("page_publish", map[string]any{"page": p.pageSummary(id), "changed": false}, 1)
	} else if !os.IsNotExist(err) {
		return pageFail("page_publish", "tool_error", "published marker could not be inspected")
	}
	if err := os.Symlink(filepath.Join("..", id), link); err != nil {
		return pageFail("page_publish", "tool_error", "page could not be published")
	}
	return pageOK("page_publish", map[string]any{"page": p.pageSummary(id), "changed": true}, 1)
}

func (p *pagesFS) unpublish(rawID string) service.ToolEnvelope {
	id, env := validatePageID("page_unpublish", rawID)
	if env != nil {
		return *env
	}
	publishedRoot, rootErr := p.publishedRoot(false)
	if rootErr != nil {
		if errors.Is(rootErr, errPublishedRoot) {
			return pageFail("page_unpublish", "conflict", rootErr.Error())
		}
		return pageFail("page_unpublish", "tool_error", "published directory could not be inspected")
	}
	link := filepath.Join(publishedRoot, id)
	info, err := os.Lstat(link)
	if os.IsNotExist(err) {
		return pageOK("page_unpublish", map[string]any{"page": p.pageSummary(id), "changed": false}, 1)
	}
	if err != nil {
		return pageFail("page_unpublish", "tool_error", "published marker could not be inspected")
	}
	if info.Mode()&os.ModeSymlink == 0 || !p.isPublished(id) {
		return pageFail("page_unpublish", "conflict", "published marker does not point to this page")
	}
	if err := os.Remove(link); err != nil {
		return pageFail("page_unpublish", "tool_error", "page could not be unpublished")
	}
	return pageOK("page_unpublish", map[string]any{"page": p.pageSummary(id), "changed": true}, 1)
}

func (p *pagesFS) pageIDs(filterID string) ([]string, error) {
	if filterID != "" {
		info, err := os.Lstat(filepath.Join(p.root, filterID))
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, nil
		}
		return []string{filterID}, nil
	}
	entries, err := os.ReadDir(p.root)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() && pageIDPattern.MatchString(entry.Name()) {
			info, err := os.Lstat(filepath.Join(p.root, entry.Name()))
			if err == nil && info.Mode()&os.ModeSymlink == 0 {
				ids = append(ids, entry.Name())
			}
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func (p *pagesFS) pageFile(id, rawPath string, createParents bool) (string, error) {
	root := filepath.Join(p.root, id)
	info, err := os.Lstat(root)
	if os.IsNotExist(err) {
		return "", os.ErrNotExist
	}
	if err != nil {
		return "", err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", errInvalidPagePath
	}
	path := strings.TrimSpace(strings.ReplaceAll(rawPath, "\\", "/"))
	if path == "" || strings.Contains(path, "\x00") {
		return "", errInvalidPagePath
	}
	rel := filepath.Clean(filepath.FromSlash(path))
	if filepath.IsAbs(rel) || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || !pageTextExtensions[strings.ToLower(filepath.Ext(rel))] {
		return "", errInvalidPagePath
	}
	parts := strings.Split(rel, string(filepath.Separator))
	current := root
	for _, part := range parts[:len(parts)-1] {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			if !createParents {
				return "", os.ErrNotExist
			}
			if err := os.Mkdir(current, 0o755); err != nil {
				return "", err
			}
			continue
		}
		if err != nil {
			return "", err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", errInvalidPagePath
		}
	}
	file := filepath.Join(root, rel)
	info, err = os.Lstat(file)
	if err == nil && (!info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0) {
		return "", errInvalidPagePath
	}
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return file, nil
}

func pagePathFail(tool string, err error) service.ToolEnvelope {
	if errors.Is(err, errInvalidPagePath) {
		return pageFail(tool, "validation", err.Error())
	}
	if errors.Is(err, os.ErrNotExist) {
		return pageFail(tool, "not_found", "page workspace or file does not exist")
	}
	return pageFail(tool, "tool_error", "page file could not be accessed")
}

func (p *pagesFS) pageSummary(id string) map[string]any {
	return map[string]any{"id": id, "path": filepath.Join(p.root, id), "published": p.isPublished(id), "published_path": filepath.Join(p.root, publishedDirName, id)}
}

func (p *pagesFS) isPublished(id string) bool {
	pageInfo, err := os.Stat(filepath.Join(p.root, id))
	if err != nil || !pageInfo.IsDir() {
		return false
	}
	publishedRoot, err := p.publishedRoot(false)
	if err != nil {
		return false
	}
	linkInfo, err := os.Stat(filepath.Join(publishedRoot, id))
	return err == nil && os.SameFile(pageInfo, linkInfo)
}

func (p *pagesFS) publishedRoot(create bool) (string, error) {
	path := filepath.Join(p.root, publishedDirName)
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		if !create {
			return path, nil
		}
		if err := os.Mkdir(path, 0o755); err != nil && !os.IsExist(err) {
			return "", err
		}
		info, err = os.Lstat(path)
	}
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", errPublishedRoot
	}
	return path, nil
}

func validatePageID(tool, raw string) (string, *service.ToolEnvelope) {
	id := strings.TrimSpace(raw)
	if !pageIDPattern.MatchString(id) {
		env := pageFail(tool, "validation", "page_id must be a lowercase slug")
		return "", &env
	}
	return id, nil
}

func truncatePageExcerpt(s string) string {
	const maxRunes = 500
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "…"
}

func pageOK(tool string, data map[string]any, count int) service.ToolEnvelope {
	return service.ToolEnvelope{OK: true, Tool: tool, Data: data, Meta: map[string]any{"truncated": false, "count": count, "data_is_untrusted": true}}
}

func pageFail(tool, code, message string) service.ToolEnvelope {
	return service.ToolEnvelope{OK: false, Tool: tool, Error: map[string]any{"code": code, "message": message}, Meta: map[string]any{"truncated": false, "count": 0, "data_is_untrusted": true}}
}
