package network

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLoginLimiterPerUser(t *testing.T) {
	lim := NewLoginLimiterWith(LoginLimits{IPMax: 100, UserMax: 3, Window: time.Minute})
	lim.now = func() time.Time { return time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC) }

	for i := 0; i < 3; i++ {
		ok, _ := lim.Allow("1.1.1.1", "owner")
		if !ok {
			t.Fatalf("attempt %d should allow", i+1)
		}
		lim.RecordFailure("1.1.1.1", "owner")
	}
	ok, retry := lim.Allow("1.1.1.1", "owner")
	if ok || retry <= 0 {
		t.Fatalf("expected deny after user max, ok=%v retry=%v", ok, retry)
	}
	lim.ClearUser("owner")
	ok, _ = lim.Allow("1.1.1.1", "owner")
	if !ok {
		t.Fatal("expected allow after ClearUser")
	}
}

func TestLoginLimiterPerIP(t *testing.T) {
	lim := NewLoginLimiterWith(LoginLimits{IPMax: 2, UserMax: 100, Window: time.Minute})
	lim.now = func() time.Time { return time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC) }

	lim.RecordFailure("10.0.0.1:1234", "a")
	lim.RecordFailure("10.0.0.1:9999", "b")
	ok, _ := lim.Allow("10.0.0.1", "c")
	if ok {
		t.Fatal("expected IP limit deny")
	}
}

func TestClientIPPrefersXForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.1")
	req.RemoteAddr = "127.0.0.1:1"
	if got := ClientIP(req); got != "203.0.113.9" {
		t.Fatalf("ClientIP=%q", got)
	}
}
