package httpdelivery_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	httpdelivery "buatpostingan/delivery/http"
)

func TestPagePreviewServesDraftFilesWithoutCaching(t *testing.T) {
	pagesRoot := t.TempDir()
	pageRoot := filepath.Join(pagesRoot, "about-us")
	if err := os.MkdirAll(filepath.Join(pageRoot, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pageRoot, "index.html"), []byte("<h1>draft</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pageRoot, "page.css"), []byte("body{color:red}"), 0o644); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	httpdelivery.MountPagePreview(mux, pagesRoot)

	for _, tc := range []struct {
		path string
		want string
		mime string
	}{
		{"/api/pages/about-us/", "<h1>draft</h1>", "text/html"},
		{"/api/pages/about-us/page.css", "body{color:red}", "text/css"},
	} {
		r := httptest.NewRequest(http.MethodGet, tc.path, nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != http.StatusOK || w.Body.String() != tc.want {
			t.Fatalf("%s: status=%d body=%q", tc.path, w.Code, w.Body.String())
		}
		if got := w.Header().Get("Cache-Control"); got != "no-store, max-age=0" {
			t.Fatalf("%s cache-control=%q", tc.path, got)
		}
		if got := w.Header().Get("Content-Type"); got == "" || got[:len(tc.mime)] != tc.mime {
			t.Fatalf("%s content-type=%q", tc.path, got)
		}
	}
}

func TestPagePreviewRejectsTraversalAndSymlink(t *testing.T) {
	pagesRoot := t.TempDir()
	pageRoot := filepath.Join(pagesRoot, "about-us")
	if err := os.MkdirAll(filepath.Join(pageRoot, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pageRoot, "index.html"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(external, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(pageRoot, "assets", "escape.txt")); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	httpdelivery.MountPagePreview(mux, pagesRoot)
	redirect := httptest.NewRecorder()
	mux.ServeHTTP(redirect, httptest.NewRequest(http.MethodGet, "/api/pages/about-us/../about-us/index.html", nil))
	if redirect.Code != http.StatusTemporaryRedirect || redirect.Header().Get("Location") != "/api/pages/about-us/index.html" {
		t.Fatalf("traversal redirect: status=%d location=%q", redirect.Code, redirect.Header().Get("Location"))
	}
	for _, path := range []string{"/api/pages/about-us/assets/escape.txt", "/api/pages/not-a-valid_slug/"} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s status=%d", path, w.Code)
		}
	}
}
