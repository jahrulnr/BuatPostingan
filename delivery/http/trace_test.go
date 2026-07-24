package httpdelivery_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpdelivery "buatpostingan/delivery/http"
	"buatpostingan/internal/pkg/logging"
)

func TestTraceMiddlewareGeneratesAndEchoes(t *testing.T) {
	t.Parallel()
	var gotCtx string
	h := httpdelivery.TraceMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCtx = logging.TraceID(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	h.ServeHTTP(rec, req)
	hdr := rec.Header().Get("X-Trace-Id")
	if hdr == "" || !strings.HasPrefix(hdr, "tr_") {
		t.Fatalf("header %q", hdr)
	}
	if gotCtx != hdr {
		t.Fatalf("ctx=%q header=%q", gotCtx, hdr)
	}
}

func TestTraceMiddlewareAcceptsIncoming(t *testing.T) {
	t.Parallel()
	var gotCtx string
	h := httpdelivery.TraceMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCtx = logging.TraceID(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Trace-Id", "tr_client_abc")
	h.ServeHTTP(rec, req)
	if rec.Header().Get("X-Trace-Id") != "tr_client_abc" || gotCtx != "tr_client_abc" {
		t.Fatalf("header=%q ctx=%q", rec.Header().Get("X-Trace-Id"), gotCtx)
	}
}

func TestTraceMiddlewareTraceparent(t *testing.T) {
	t.Parallel()
	want := "0af7651916cd43dd8448eb211c80319c"
	h := httpdelivery.TraceMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if logging.TraceID(r.Context()) != want {
			t.Errorf("ctx %q", logging.TraceID(r.Context()))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("traceparent", "00-"+want+"-b7ad6b7169203331-01")
	h.ServeHTTP(rec, req)
	if rec.Header().Get("X-Trace-Id") != want {
		t.Fatalf("header %q", rec.Header().Get("X-Trace-Id"))
	}
}
