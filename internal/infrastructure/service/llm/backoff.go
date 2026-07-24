package llm

import (
	"context"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// retryPolicy computes the delay between transient LLM retry attempts.
//
// Base is grown exponentially per retry and perturbed by bounded jitter, then
// capped at maxDelay. A provider-supplied Retry-After takes precedence (capped
// by maxDelay, no jitter — honor the server's request exactly but never longer
// than our own ceiling). rand + sleep are injectable so tests stay deterministic
// and fast (no real sleeping).
type retryPolicy struct {
	baseDelay time.Duration
	maxDelay  time.Duration
	jitter    float64 // fraction in [0,1]; ±jitter around the exponential base

	rand  func() float64                             // [0,1)
	sleep func(context.Context, time.Duration) error // nil → real timer
}

func newRetryPolicy(cfg Config) *retryPolicy {
	base := time.Duration(cfg.RetryBaseDelayMS) * time.Millisecond
	if base <= 0 {
		base = 250 * time.Millisecond
	}
	max := time.Duration(cfg.RetryMaxDelayMS) * time.Millisecond
	if max <= 0 {
		max = 5000 * time.Millisecond
	}
	if max < base {
		max = base
	}
	j := cfg.RetryJitter
	if j < 0 {
		j = 0
	}
	if j > 1 {
		j = 1
	}
	return &retryPolicy{baseDelay: base, maxDelay: max, jitter: j, rand: rand.Float64}
}

// delay returns the wait before retry number retryNum (1-based). retryAfter,
// when > 0, is a provider-requested delay (Retry-After) and wins, capped by max.
func (p *retryPolicy) delay(retryNum int, retryAfter time.Duration) time.Duration {
	max := p.maxDelay
	if max <= 0 {
		max = 5 * time.Second
	}
	if retryAfter > 0 {
		if retryAfter > max {
			return max
		}
		return retryAfter
	}
	base := p.baseDelay
	if base <= 0 {
		base = 250 * time.Millisecond
	}
	if retryNum < 1 {
		retryNum = 1
	}
	// Cap the exponent so a large budget cannot overflow the float multiply;
	// the result is clamped by max anyway.
	exp := retryNum - 1
	if exp > 20 {
		exp = 20
	}
	d := float64(base) * math.Pow(2, float64(exp))
	if p.jitter > 0 {
		r := 0.5
		if p.rand != nil {
			r = p.rand()
		}
		d *= 1 + p.jitter*(2*r-1) // [1-jitter, 1+jitter)
	}
	if d < 0 {
		d = 0
	}
	out := time.Duration(d)
	if out > max {
		out = max
	}
	return out
}

// wait sleeps d honoring context cancellation. It never sleeps once ctx is done.
func (p *retryPolicy) wait(ctx context.Context, d time.Duration) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if d <= 0 {
		return nil
	}
	if p.sleep != nil {
		return p.sleep(ctx, d)
	}
	t := time.NewTimer(d)
	defer t.Stop()
	if ctx == nil {
		<-t.C
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// parseRetryAfter reads an HTTP Retry-After header value (delta-seconds or an
// HTTP-date) relative to now. Returns (0,false) when absent/unparseable.
func parseRetryAfter(v string, now time.Time) (time.Duration, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs <= 0 {
			return 0, true
		}
		return time.Duration(secs) * time.Second, true
	}
	if t, err := http.ParseTime(v); err == nil {
		d := t.Sub(now)
		if d < 0 {
			d = 0
		}
		return d, true
	}
	return 0, false
}
