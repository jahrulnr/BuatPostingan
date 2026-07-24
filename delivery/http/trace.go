package httpdelivery

import (
	"net/http"

	"buatpostingan/internal/pkg/logging"
)

// TraceMiddleware ensures every request has a trace id in context and echoes
// X-Trace-Id on the response. Accepts incoming X-Trace-Id or W3C traceparent.
func TraceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := logging.ResolveIncoming(r.Header.Get("X-Trace-Id"), r.Header.Get("traceparent"))
		if id == "" {
			id = logging.NewTraceID()
		}
		ctx := logging.WithTraceID(r.Context(), id)
		w.Header().Set("X-Trace-Id", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
