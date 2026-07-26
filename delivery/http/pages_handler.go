package httpdelivery

import (
	"errors"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var pagePreviewIDPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,78}[a-z0-9])?$`)

// MountPagePreview serves draft static pages from pagesRoot. It deliberately
// exposes a read-only, jailed route rather than the host filesystem.
func MountPagePreview(mux *http.ServeMux, pagesRoot string) {
	root, err := pagePreviewRoot(pagesRoot)
	if err != nil {
		root = ""
	}
	mux.HandleFunc("GET /api/pages/", func(w http.ResponseWriter, r *http.Request) {
		servePagePreview(w, r, root)
	})
}

func pagePreviewRoot(raw string) (string, error) {
	abs, err := filepath.Abs(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(real)
	if err != nil || !info.IsDir() {
		return "", errors.New("pages root unavailable")
	}
	return real, nil
}

func servePagePreview(w http.ResponseWriter, r *http.Request, root string) {
	if root == "" {
		http.NotFound(w, r)
		return
	}
	rel := strings.TrimPrefix(r.URL.Path, "/api/pages/")
	parts := strings.Split(rel, "/")
	if len(parts) == 0 || !pagePreviewIDPattern.MatchString(parts[0]) {
		http.NotFound(w, r)
		return
	}
	pageRoot := filepath.Join(root, parts[0])
	pageInfo, err := os.Lstat(pageRoot)
	if err != nil || !pageInfo.IsDir() || pageInfo.Mode()&os.ModeSymlink != 0 {
		http.NotFound(w, r)
		return
	}
	assetPath := "index.html"
	if len(parts) > 1 && strings.Join(parts[1:], "/") != "" {
		assetPath = strings.Join(parts[1:], "/")
	}
	file, err := previewFile(pageRoot, assetPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Cache-Control", "no-store, max-age=0")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'self' data:; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; connect-src 'self'; object-src 'none'; base-uri 'none'")
	if contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(info.Name()))); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

func previewFile(pageRoot, rawPath string) (*os.File, error) {
	path := strings.TrimSpace(strings.ReplaceAll(rawPath, "\\", "/"))
	if path == "" || strings.Contains(path, "\x00") {
		return nil, errors.New("invalid page preview path")
	}
	rel := filepath.Clean(filepath.FromSlash(path))
	if filepath.IsAbs(rel) || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, errors.New("invalid page preview path")
	}
	parts := strings.Split(rel, string(filepath.Separator))
	current := pageRoot
	for _, part := range parts {
		if part == "." || strings.HasPrefix(part, ".") {
			return nil, errors.New("invalid page preview path")
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("invalid page preview path")
		}
	}
	file, err := os.Open(current)
	if err != nil {
		return nil, err
	}
	return file, nil
}
