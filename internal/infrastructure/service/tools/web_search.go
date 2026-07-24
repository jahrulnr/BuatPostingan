package tools

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"buatpostingan/internal/domain/service"

	"github.com/jahrulnr/searchwire"
)

const (
	defaultWebSearchLimit   = 5
	maxWebSearchLimit       = 10
	defaultWebSearchTimeout = 12 * time.Second
)

// WebSearcher is the search port used by web_search (searchwire in production).
type WebSearcher interface {
	Search(ctx context.Context, query string, limit int) (*WebSearchResponse, error)
}

// WebSearchResponse is a host-friendly view of metasearch output.
type WebSearchResponse struct {
	Query   string
	Results []WebSearchResult
	Errors  []WebSearchSourceError
}

// WebSearchResult is one ranked hit.
type WebSearchResult struct {
	Title   string
	URL     string
	Snippet string
	Sources []string
	Score   float64
}

// WebSearchSourceError records a partial source failure.
type WebSearchSourceError struct {
	Source string
	Error  string
}

type searchwireSearcher struct {
	timeout     time.Duration
	githubToken string
}

func newSearchwireSearcher(timeout time.Duration, githubToken string) *searchwireSearcher {
	if timeout <= 0 {
		timeout = defaultWebSearchTimeout
	}
	return &searchwireSearcher{timeout: timeout, githubToken: githubToken}
}

func (s *searchwireSearcher) resolveGitHubToken() string {
	if t := strings.TrimSpace(s.githubToken); t != "" {
		return t
	}
	return githubTokenFromEnv()
}

func (s *searchwireSearcher) Search(ctx context.Context, query string, limit int) (*WebSearchResponse, error) {
	if limit < 1 {
		limit = defaultWebSearchLimit
	}
	if limit > maxWebSearchLimit {
		limit = maxWebSearchLimit
	}
	cfg := searchwire.Config{
		Limit:   limit,
		Timeout: s.timeout,
		GitHub: searchwire.GitHubConfig{
			Token: s.resolveGitHubToken(),
		},
	}
	resp, err := searchwire.New(cfg).Search(ctx, query)
	if err != nil {
		return nil, err
	}
	out := &WebSearchResponse{Query: resp.Query}
	for _, r := range resp.Results {
		out.Results = append(out.Results, WebSearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Snippet,
			Sources: r.Sources,
			Score:   r.Score,
		})
	}
	for _, e := range resp.Errors {
		out.Errors = append(out.Errors, WebSearchSourceError{
			Source: e.Source,
			Error:  e.Error,
		})
	}
	return out, nil
}

func githubTokenFromEnv() string {
	if t := strings.TrimSpace(os.Getenv("BP_GITHUB_TOKEN")); t != "" {
		return t
	}
	return strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
}

func (r *Registry) execWebSearch(ctx context.Context, args map[string]any) service.ToolEnvelope {
	query := strings.TrimSpace(asString(args["query"]))
	if query == "" {
		return service.ToolEnvelope{
			OK:   false,
			Tool: "web_search",
			Data: nil,
			Error: map[string]any{
				"code":    "validation",
				"message": "query required",
			},
			Meta: map[string]any{
				"truncated":         false,
				"data_is_untrusted": true,
			},
		}
	}
	limit := clamp(asInt(args["limit"], defaultWebSearchLimit), 1, maxWebSearchLimit)

	searcher := r.webSearch
	if searcher == nil {
		searcher = newSearchwireSearcher(0, r.githubToken)
	}

	resp, err := searcher.Search(ctx, query, limit)
	if err != nil {
		msg := "web search failed"
		code := "tool_error"
		if errors.Is(err, searchwire.ErrEmptyQuery) {
			code = "validation"
			msg = "query required"
		} else {
			var se *searchwire.SearchError
			if errors.As(err, &se) && se != nil {
				msg = "all search sources failed"
			}
		}
		return service.ToolEnvelope{
			OK:   false,
			Tool: "web_search",
			Data: nil,
			Error: map[string]any{
				"code":    code,
				"message": msg,
			},
			Meta: map[string]any{
				"truncated":         false,
				"data_is_untrusted": true,
			},
		}
	}

	results := make([]map[string]any, 0, len(resp.Results))
	for _, hit := range resp.Results {
		results = append(results, map[string]any{
			"title":   hit.Title,
			"url":     hit.URL,
			"snippet": hit.Snippet,
			"sources": hit.Sources,
			"score":   hit.Score,
		})
	}
	sourceErrors := make([]map[string]any, 0, len(resp.Errors))
	for _, e := range resp.Errors {
		sourceErrors = append(sourceErrors, map[string]any{
			"source": e.Source,
			"error":  e.Error,
		})
	}

	return service.ToolEnvelope{
		OK:   true,
		Tool: "web_search",
		Data: map[string]any{
			"query":         resp.Query,
			"results":       results,
			"source_errors": sourceErrors,
		},
		Error: nil,
		Meta: map[string]any{
			"truncated":         len(results) >= limit,
			"count":             len(results),
			"data_is_untrusted": true,
		},
	}
}
