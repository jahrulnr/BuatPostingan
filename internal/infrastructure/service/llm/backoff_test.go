package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"buatpostingan/internal/config"
)

func TestRetryPolicyDelayExponentialAndCap(t *testing.T) {
	p := &retryPolicy{baseDelay: 100 * time.Millisecond, maxDelay: 1000 * time.Millisecond, jitter: 0}
	cases := []struct {
		retryNum int
		want     time.Duration
	}{
		{1, 100 * time.Millisecond},
		{2, 200 * time.Millisecond},
		{3, 400 * time.Millisecond},
		{4, 800 * time.Millisecond},
		{5, 1000 * time.Millisecond}, // capped
		{9, 1000 * time.Millisecond}, // capped
	}
	for _, tc := range cases {
		if got := p.delay(tc.retryNum, 0); got != tc.want {
			t.Fatalf("delay(%d)=%v want %v", tc.retryNum, got, tc.want)
		}
	}
}

func TestRetryPolicyRetryAfterWinsCapped(t *testing.T) {
	p := &retryPolicy{baseDelay: 100 * time.Millisecond, maxDelay: 1000 * time.Millisecond, jitter: 0.5}
	if got := p.delay(1, 500*time.Millisecond); got != 500*time.Millisecond {
		t.Fatalf("retry-after honored got %v", got)
	}
	if got := p.delay(1, 9*time.Second); got != 1000*time.Millisecond {
		t.Fatalf("retry-after capped by max got %v", got)
	}
}

func TestRetryPolicyJitterBounds(t *testing.T) {
	base := 100 * time.Millisecond
	pLow := &retryPolicy{baseDelay: base, maxDelay: time.Minute, jitter: 0.2, rand: func() float64 { return 0 }}
	if got := pLow.delay(1, 0); got != 80*time.Millisecond {
		t.Fatalf("jitter low bound got %v want 80ms", got)
	}
	pHigh := &retryPolicy{baseDelay: base, maxDelay: time.Minute, jitter: 0.2, rand: func() float64 { return 1 }}
	if got := pHigh.delay(1, 0); got != 120*time.Millisecond {
		t.Fatalf("jitter high bound got %v want 120ms", got)
	}
}

func TestRetryPolicyWaitRespectsContext(t *testing.T) {
	p := &retryPolicy{baseDelay: time.Second, maxDelay: time.Minute}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	if err := p.wait(ctx, 5*time.Second); err == nil {
		t.Fatal("want ctx error")
	}
	if time.Since(start) > 200*time.Millisecond {
		t.Fatal("must not sleep after ctx done")
	}
	// zero delay → nil even with live ctx
	if err := p.wait(context.Background(), 0); err != nil {
		t.Fatalf("zero delay err %v", err)
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)
	if d, ok := parseRetryAfter("2", now); !ok || d != 2*time.Second {
		t.Fatalf("seconds %v %v", d, ok)
	}
	if _, ok := parseRetryAfter("", now); ok {
		t.Fatal("empty should be false")
	}
	if _, ok := parseRetryAfter("garbage", now); ok {
		t.Fatal("garbage should be false")
	}
	if d, ok := parseRetryAfter("-3", now); !ok || d != 0 {
		t.Fatalf("negative → 0 true, got %v %v", d, ok)
	}
	future := now.Add(30 * time.Second).UTC().Format(http.TimeFormat)
	if d, ok := parseRetryAfter(future, now); !ok || d != 30*time.Second {
		t.Fatalf("http-date %v %v", d, ok)
	}
	past := now.Add(-30 * time.Second).UTC().Format(http.TimeFormat)
	if d, ok := parseRetryAfter(past, now); !ok || d != 0 {
		t.Fatalf("past date → 0, got %v %v", d, ok)
	}
}

// TestRouterHonorsRetryAfterHeader confirms a provider Retry-After drives the
// backoff delay (capped by max) between transient attempts.
func TestRouterHonorsRetryAfterHeader(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits == 1 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"slow"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object":  "chat.completion",
			"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": "ok"}}},
		})
	}))
	defer srv.Close()

	root := t.TempDir()
	cfg := Config{
		StorageRoot: root, Strategy: "failover", ActiveProvider: "A",
		TotalAttemptBudget: 4, RetryStatuses: []int{429},
		RetryBaseDelayMS: 100, RetryMaxDelayMS: 5000, RetryJitter: 0,
		Providers: map[string]config.LLMProvider{
			"A": {ID: "A", Enabled: true, APIKey: "k", BaseURL: srv.URL, Model: "m", API: "chat", TimeoutSec: 5, MaxAttempts: 2, MaxOutputTokens: 32},
		},
	}
	r := NewRouter(cfg, NewClient(cfg))
	var gotDelay time.Duration
	r.retry.sleep = func(_ context.Context, d time.Duration) error { gotDelay = d; return nil }
	res, err := r.Chat(context.Background(), []map[string]any{{"role": "user", "content": "hi"}}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "ok" {
		t.Fatalf("%#v", res)
	}
	if gotDelay != 2*time.Second {
		t.Fatalf("expected Retry-After 2s backoff, got %v", gotDelay)
	}
}

// TestClientParsesRetryAfterOnTransient verifies the client surfaces Retry-After.
func TestClientParsesRetryAfterOnTransient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "3")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, "slow")
	}))
	defer srv.Close()
	c := NewClient(Config{RetryStatuses: []int{429}})
	p := config.LLMProvider{ID: "P", BaseURL: srv.URL, APIKey: "k", TimeoutSec: 5}
	_, err := c.postJSON(context.Background(), p, "chat/completions", map[string]any{"model": "m"})
	le, ok := err.(*Error)
	if !ok || !le.Transient || le.RetryAfter != 3*time.Second {
		t.Fatalf("want transient with RetryAfter=3s, got %#v", err)
	}
}
