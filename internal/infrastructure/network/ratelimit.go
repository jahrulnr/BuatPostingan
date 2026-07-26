// Package network holds shared edge-network utilities (rate limits, client IP).
package network

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// LoginLimiter throttles login attempts by client IP and username.
// In-memory only — fine for single-process deploy (Docker/compose MVP).
type LoginLimiter struct {
	mu sync.Mutex

	ipMax    int
	userMax  int
	window   time.Duration
	ipHits   map[string][]time.Time
	userHits map[string][]time.Time
	now      func() time.Time
}

const (
	defaultLoginIPMax   = 20
	defaultLoginUserMax = 5
	defaultLoginWindow  = 15 * time.Minute
	maxLoginLimiterKeys = 10_000
)

// LoginLimits configures a LoginLimiter. Zero values use defaults.
type LoginLimits struct {
	IPMax   int
	UserMax int
	Window  time.Duration
}

func NewLoginLimiter() *LoginLimiter {
	return NewLoginLimiterWith(LoginLimits{})
}

func NewLoginLimiterWith(limits LoginLimits) *LoginLimiter {
	if limits.IPMax <= 0 {
		limits.IPMax = defaultLoginIPMax
	}
	if limits.UserMax <= 0 {
		limits.UserMax = defaultLoginUserMax
	}
	if limits.Window <= 0 {
		limits.Window = defaultLoginWindow
	}
	return &LoginLimiter{
		ipMax:    limits.IPMax,
		userMax:  limits.UserMax,
		window:   limits.Window,
		ipHits:   make(map[string][]time.Time),
		userHits: make(map[string][]time.Time),
		now:      time.Now,
	}
}

// Allow reports whether a login attempt may proceed. On deny, retryAfter is a
// positive duration suitable for Retry-After.
func (l *LoginLimiter) Allow(ip, username string) (ok bool, retryAfter time.Duration) {
	if l == nil {
		return true, 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now().UTC()
	cutoff := now.Add(-l.window)

	ip = NormalizeIP(ip)
	username = strings.ToLower(strings.TrimSpace(username))

	l.ipHits[ip] = pruneHits(l.ipHits[ip], cutoff)
	if username != "" {
		l.userHits[username] = pruneHits(l.userHits[username], cutoff)
	}

	if len(l.ipHits[ip]) >= l.ipMax {
		return false, retryAfterFrom(l.ipHits[ip][0], l.window, now)
	}
	if username != "" && len(l.userHits[username]) >= l.userMax {
		return false, retryAfterFrom(l.userHits[username][0], l.window, now)
	}
	return true, 0
}

// RecordFailure records a failed login attempt against IP and username.
func (l *LoginLimiter) RecordFailure(ip, username string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now().UTC()
	cutoff := now.Add(-l.window)
	ip = NormalizeIP(ip)
	username = strings.ToLower(strings.TrimSpace(username))

	l.gcLocked(cutoff)
	l.ipHits[ip] = append(pruneHits(l.ipHits[ip], cutoff), now)
	if username != "" {
		l.userHits[username] = append(pruneHits(l.userHits[username], cutoff), now)
	}
}

// ClearUser clears the per-username failure window after a successful login.
func (l *LoginLimiter) ClearUser(username string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.userHits, strings.ToLower(strings.TrimSpace(username)))
}

func (l *LoginLimiter) gcLocked(cutoff time.Time) {
	if len(l.ipHits)+len(l.userHits) < maxLoginLimiterKeys {
		return
	}
	for k, hits := range l.ipHits {
		hits = pruneHits(hits, cutoff)
		if len(hits) == 0 {
			delete(l.ipHits, k)
		} else {
			l.ipHits[k] = hits
		}
	}
	for k, hits := range l.userHits {
		hits = pruneHits(hits, cutoff)
		if len(hits) == 0 {
			delete(l.userHits, k)
		} else {
			l.userHits[k] = hits
		}
	}
}

func pruneHits(hits []time.Time, cutoff time.Time) []time.Time {
	i := 0
	for i < len(hits) && !hits[i].After(cutoff) {
		i++
	}
	if i == 0 {
		return hits
	}
	out := make([]time.Time, len(hits)-i)
	copy(out, hits[i:])
	return out
}

func retryAfterFrom(oldest time.Time, window time.Duration, now time.Time) time.Duration {
	until := oldest.Add(window).Sub(now)
	if until < time.Second {
		return time.Second
	}
	return until.Round(time.Second)
}

// NormalizeIP strips an optional host:port to a bare IP / host string.
func NormalizeIP(ip string) string {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return "unknown"
	}
	if host, _, err := net.SplitHostPort(ip); err == nil {
		return host
	}
	return ip
}

// ClientIP returns the best-effort client address for rate limiting.
// Prefer the first X-Forwarded-For hop (nginx sets this); fall back to RemoteAddr.
func ClientIP(r *http.Request) string {
	if r == nil {
		return "unknown"
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			if ip := strings.TrimSpace(parts[0]); ip != "" {
				return ip
			}
		}
	}
	return r.RemoteAddr
}
