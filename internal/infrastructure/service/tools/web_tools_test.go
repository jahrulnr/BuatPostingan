package tools_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"buatpostingan/internal/domain/service"
	"buatpostingan/internal/infrastructure/service/docs"
	"buatpostingan/internal/infrastructure/service/tools"
)

type fakeWebSearcher struct {
	resp *tools.WebSearchResponse
	err  error
	lastQuery string
	lastLimit int
}

func (f *fakeWebSearcher) Search(_ context.Context, query string, limit int) (*tools.WebSearchResponse, error) {
	f.lastQuery = query
	f.lastLimit = limit
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

func TestWebSearchValidationAndEnvelope(t *testing.T) {
	repoRoot := findRepoRoot(t)
	docsRoot := filepath.Join(repoRoot, "resources", "webchat", "docs")
	toolsRoot := filepath.Join(repoRoot, "resources", "webchat", "tools")
	storage := t.TempDir()

	idx, err := docs.NewIndex(docsRoot, storage, docs.Options{})
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	fake := &fakeWebSearcher{
		resp: &tools.WebSearchResponse{
			Query: "go context",
			Results: []tools.WebSearchResult{
				{Title: "Context", URL: "https://example.com/ctx", Snippet: "cancel", Sources: []string{"brave"}, Score: 1},
			},
		},
	}
	reg, err := tools.NewRegistry(toolsRoot, idx, tools.Options{WebSearch: fake})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	env, err := reg.Execute(context.Background(), service.ToolCall{
		Name:      "web_search",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if env.OK {
		t.Fatal("expected validation failure")
	}
	if env.Error["code"] != "validation" {
		t.Fatalf("code=%v", env.Error)
	}
	if env.Meta["data_is_untrusted"] != true {
		t.Fatalf("meta=%v", env.Meta)
	}

	env, err = reg.Execute(context.Background(), service.ToolCall{
		Name: "web_search",
		Arguments: map[string]any{
			"query": "go context",
			"limit": "3",
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !env.OK {
		t.Fatalf("expected ok: %+v", env)
	}
	if env.Tool != "web_search" {
		t.Fatalf("tool=%s", env.Tool)
	}
	if fake.lastQuery != "go context" || fake.lastLimit != 3 {
		t.Fatalf("fake got query=%q limit=%d", fake.lastQuery, fake.lastLimit)
	}
	data, _ := env.Data.(map[string]any)
	results, _ := data["results"].([]map[string]any)
	if len(results) != 1 {
		// JSON round-trip via map may use []any
		if raw, ok := data["results"].([]any); ok && len(raw) == 1 {
			// ok
		} else {
			t.Fatalf("results=%#v", data["results"])
		}
	}
	if env.Meta["data_is_untrusted"] != true {
		t.Fatalf("meta=%v", env.Meta)
	}
	if env.Meta["count"] != 1 {
		t.Fatalf("count=%v", env.Meta["count"])
	}
}

func TestWebSearchAllSourcesFailed(t *testing.T) {
	repoRoot := findRepoRoot(t)
	docsRoot := filepath.Join(repoRoot, "resources", "webchat", "docs")
	toolsRoot := filepath.Join(repoRoot, "resources", "webchat", "tools")
	idx, err := docs.NewIndex(docsRoot, t.TempDir(), docs.Options{})
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	fake := &fakeWebSearcher{err: errors.New("all search sources failed")}
	reg, err := tools.NewRegistry(toolsRoot, idx, tools.Options{WebSearch: fake})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	env, err := reg.Execute(context.Background(), service.ToolCall{
		Name:      "web_search",
		Arguments: map[string]any{"query": "x"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if env.OK || env.Error["code"] != "tool_error" {
		t.Fatalf("env=%+v", env)
	}
}

func TestWebFetchHappyPathAndSSRF(t *testing.T) {
	repoRoot := findRepoRoot(t)
	docsRoot := filepath.Join(repoRoot, "resources", "webchat", "docs")
	toolsRoot := filepath.Join(repoRoot, "resources", "webchat", "tools")
	idx, err := docs.NewIndex(docsRoot, t.TempDir(), docs.Options{})
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, `<html><head><title>Hello Page</title></head><body><h1>Hi</h1><p>World content here.</p><script>alert(1)</script></body></html>`)
	}))
	t.Cleanup(srv.Close)

	reg, err := tools.NewRegistry(toolsRoot, idx, tools.Options{
		FetchClient: srv.Client(),
	})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	env, err := reg.Execute(context.Background(), service.ToolCall{
		Name:      "web_fetch",
		Arguments: map[string]any{"url": srv.URL + "/page"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !env.OK {
		t.Fatalf("expected ok: %+v", env)
	}
	data, _ := env.Data.(map[string]any)
	if data["title"] != "Hello Page" {
		t.Fatalf("title=%v", data["title"])
	}
	text, _ := data["text"].(string)
	if !strings.Contains(text, "World content") {
		t.Fatalf("text=%q", text)
	}
	if strings.Contains(text, "alert") {
		t.Fatalf("script leaked into text: %q", text)
	}
	if env.Meta["data_is_untrusted"] != true {
		t.Fatalf("meta=%v", env.Meta)
	}

	// SSRF blocks — use default client (not httptest) so DialContext policy applies.
	regStrict, err := tools.NewRegistry(toolsRoot, idx, tools.Options{})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	for _, bad := range []string{
		"file:///etc/passwd",
		"http://127.0.0.1/",
		"http://localhost/",
		"http://169.254.169.254/latest/meta-data/",
		"http://192.168.1.1/",
		"http://10.0.0.1/",
		"http://[::1]/",
		"ftp://example.com/",
	} {
		env, err := regStrict.Execute(context.Background(), service.ToolCall{
			Name:      "web_fetch",
			Arguments: map[string]any{"url": bad},
		})
		if err != nil {
			t.Fatalf("Execute %s: %v", bad, err)
		}
		if env.OK {
			t.Fatalf("%s: expected block, got ok", bad)
		}
		code, _ := env.Error["code"].(string)
		if code != "ssrf_blocked" && code != "validation" {
			// file/ftp → ssrf_blocked via parseFetchURL
			if code != "ssrf_blocked" {
				t.Fatalf("%s: want ssrf_blocked, got %+v", bad, env.Error)
			}
		}
	}

	env, err = regStrict.Execute(context.Background(), service.ToolCall{
		Name:      "web_fetch",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if env.OK || env.Error["code"] != "validation" {
		t.Fatalf("empty url: %+v", env)
	}
}

func TestWebFetchUnsupportedContentType(t *testing.T) {
	repoRoot := findRepoRoot(t)
	docsRoot := filepath.Join(repoRoot, "resources", "webchat", "docs")
	toolsRoot := filepath.Join(repoRoot, "resources", "webchat", "tools")
	idx, err := docs.NewIndex(docsRoot, t.TempDir(), docs.Options{})
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte{0x00, 0x01})
	}))
	t.Cleanup(srv.Close)

	reg, err := tools.NewRegistry(toolsRoot, idx, tools.Options{FetchClient: srv.Client()})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	env, err := reg.Execute(context.Background(), service.ToolCall{
		Name:      "web_fetch",
		Arguments: map[string]any{"url": srv.URL},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if env.OK || env.Error["code"] != "unsupported_content_type" {
		t.Fatalf("env=%+v", env)
	}
}

func TestSchemasIncludeWebTools(t *testing.T) {
	repoRoot := findRepoRoot(t)
	docsRoot := filepath.Join(repoRoot, "resources", "webchat", "docs")
	toolsRoot := filepath.Join(repoRoot, "resources", "webchat", "tools")
	idx, err := docs.NewIndex(docsRoot, t.TempDir(), docs.Options{})
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	reg, err := tools.NewRegistry(toolsRoot, idx, tools.Options{})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	schemas, err := reg.Schemas(context.Background())
	if err != nil {
		t.Fatalf("Schemas: %v", err)
	}
	if len(schemas) != len(tools.Allowlist) {
		t.Fatalf("want %d schemas, got %d", len(tools.Allowlist), len(schemas))
	}
	names := map[string]bool{}
	for _, s := range schemas {
		fn, _ := s["function"].(map[string]any)
		name, _ := fn["name"].(string)
		names[name] = true
	}
	for _, want := range []string{"web_search", "web_fetch", "page_list", "page_search", "page_create", "page_edit", "page_read", "page_publish", "page_unpublish"} {
		if !names[want] {
			t.Fatalf("missing schema %s in %#v", want, names)
		}
	}
}
