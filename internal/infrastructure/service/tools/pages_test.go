package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"buatpostingan/internal/domain/service"
	"buatpostingan/internal/infrastructure/service/docs"
	"buatpostingan/internal/infrastructure/service/tools"
)

func TestPageLifecycleUsesPublishedSymlink(t *testing.T) {
	pagesRoot := t.TempDir()
	pageRoot := filepath.Join(pagesRoot, "about-us")
	if err := os.MkdirAll(pageRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pageRoot, "index.html"), []byte("<h1>About BuatPostingan</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}

	reg := newPageRegistry(t, pagesRoot)
	publish := callPageTool(t, reg, "page_publish", map[string]any{"page_id": "about-us"})
	if !publish.OK {
		t.Fatalf("publish=%+v", publish)
	}
	link := filepath.Join(pagesRoot, ".published", "about-us")
	info, err := os.Lstat(link)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("published marker must be a symlink: info=%v err=%v", info, err)
	}
	target, err := os.Readlink(link)
	if err != nil || target != filepath.Join("..", "about-us") {
		t.Fatalf("target=%q err=%v", target, err)
	}

	listed := callPageTool(t, reg, "page_list", nil)
	pages := listed.Data.(map[string]any)["pages"].([]map[string]any)
	if len(pages) != 1 || pages[0]["id"] != "about-us" || pages[0]["published"] != true {
		t.Fatalf("pages=%#v", pages)
	}

	search := callPageTool(t, reg, "page_search", map[string]any{"query": "BuatPostingan"})
	matches := search.Data.(map[string]any)["matches"].([]map[string]any)
	if len(matches) != 1 || matches[0]["page_id"] != "about-us" || matches[0]["published"] != true {
		t.Fatalf("matches=%#v", matches)
	}

	unpublish := callPageTool(t, reg, "page_unpublish", map[string]any{"page_id": "about-us"})
	if !unpublish.OK {
		t.Fatalf("unpublish=%+v", unpublish)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatalf("published symlink still exists: %v", err)
	}
}

func TestPageCreateEditAndReadStayInsidePageWorkspace(t *testing.T) {
	pagesRoot := t.TempDir()
	reg := newPageRegistry(t, pagesRoot)

	created := callPageTool(t, reg, "page_create", map[string]any{"page_id": "launch", "title": "Launch page"})
	if !created.OK {
		t.Fatalf("create=%+v", created)
	}
	for _, name := range []string{"index.html", "page.css", "page.js"} {
		if _, err := os.Stat(filepath.Join(pagesRoot, "launch", name)); err != nil {
			t.Fatalf("starter file %s: %v", name, err)
		}
	}
	if info, err := os.Stat(filepath.Join(pagesRoot, "launch", "assets")); err != nil || !info.IsDir() {
		t.Fatalf("assets directory: info=%v err=%v", info, err)
	}

	overwrite := callPageTool(t, reg, "page_edit", map[string]any{"page_id": "launch", "path": "index.html", "content": "<main>draft</main>"})
	if !overwrite.OK {
		t.Fatalf("overwrite=%+v", overwrite)
	}
	appendResult := callPageTool(t, reg, "page_edit", map[string]any{"page_id": "launch", "path": "index.html", "content": "\n<footer>done</footer>", "mode": "append"})
	if !appendResult.OK {
		t.Fatalf("append=%+v", appendResult)
	}
	replaced := callPageTool(t, reg, "page_edit", map[string]any{"page_id": "launch", "path": "index.html", "mode": "replace", "old_string": "draft", "content": "reviewed"})
	if !replaced.OK {
		t.Fatalf("replace=%+v", replaced)
	}
	read := callPageTool(t, reg, "page_read", map[string]any{"page_id": "launch", "path": "index.html"})
	if !read.OK || read.Data.(map[string]any)["content"] != "<main>reviewed</main>\n<footer>done</footer>" {
		t.Fatalf("read=%+v", read)
	}

	traversal := callPageTool(t, reg, "page_edit", map[string]any{"page_id": "launch", "path": "../outside.html", "content": "no"})
	if traversal.OK || traversal.Error["code"] != "validation" {
		t.Fatalf("traversal=%+v", traversal)
	}
	external := filepath.Join(t.TempDir(), "outside.html")
	if err := os.WriteFile(external, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(pagesRoot, "launch", "assets", "escape.html")); err != nil {
		t.Fatal(err)
	}
	symlink := callPageTool(t, reg, "page_read", map[string]any{"page_id": "launch", "path": "assets/escape.html"})
	if symlink.OK || symlink.Error["code"] != "validation" {
		t.Fatalf("symlink must be rejected: %+v", symlink)
	}
}

func TestPageToolsRejectTraversalAndForeignPublishedLink(t *testing.T) {
	pagesRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(pagesRoot, "about-us"), 0o755); err != nil {
		t.Fatal(err)
	}
	reg := newPageRegistry(t, pagesRoot)

	badID := callPageTool(t, reg, "page_publish", map[string]any{"page_id": "../outside"})
	if badID.OK || badID.Error["code"] != "validation" {
		t.Fatalf("traversal should fail validation: %+v", badID)
	}

	publishedRoot := filepath.Join(pagesRoot, ".published")
	if err := os.MkdirAll(publishedRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/tmp", filepath.Join(publishedRoot, "about-us")); err != nil {
		t.Fatal(err)
	}
	foreign := callPageTool(t, reg, "page_publish", map[string]any{"page_id": "about-us"})
	if foreign.OK || foreign.Error["code"] != "conflict" {
		t.Fatalf("foreign link should fail conflict: %+v", foreign)
	}
}

func TestPagePublishRejectsPublishedDirectorySymlink(t *testing.T) {
	pagesRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(pagesRoot, "about-us"), 0o755); err != nil {
		t.Fatal(err)
	}
	externalRoot := t.TempDir()
	if err := os.Symlink(externalRoot, filepath.Join(pagesRoot, ".published")); err != nil {
		t.Fatal(err)
	}

	env := callPageTool(t, newPageRegistry(t, pagesRoot), "page_publish", map[string]any{"page_id": "about-us"})
	if env.OK || env.Error["code"] != "conflict" {
		t.Fatalf("published directory symlink should fail conflict: %+v", env)
	}
	if _, err := os.Lstat(filepath.Join(externalRoot, "about-us")); !os.IsNotExist(err) {
		t.Fatalf("tool must not create a marker outside pages root: %v", err)
	}
}

func newPageRegistry(t *testing.T, pagesRoot string) *tools.Registry {
	t.Helper()
	repoRoot := findRepoRoot(t)
	idx, err := docs.NewIndex(filepath.Join(repoRoot, "resources", "webchat", "docs"), t.TempDir(), docs.Options{})
	if err != nil {
		t.Fatal(err)
	}
	reg, err := tools.NewRegistry(filepath.Join(repoRoot, "resources", "webchat", "tools"), idx, tools.Options{PagesRoot: pagesRoot})
	if err != nil {
		t.Fatal(err)
	}
	return reg
}

func callPageTool(t *testing.T, reg *tools.Registry, name string, args map[string]any) service.ToolEnvelope {
	t.Helper()
	env, err := reg.Execute(context.Background(), service.ToolCall{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return env
}
