package httpdelivery

import (
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"buatpostingan/internal/application/usecase/webchat"
	"buatpostingan/internal/config"
)

// NewServer wires HTTP routes: API + static web/.
func NewServer(cfg config.Config, uc *webchat.Usecase) *http.Server {
	mux := http.NewServeMux()

	webchatHandler := NewWebchatHandler(uc)
	webchatHandler.Register(mux)

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	webRoot, err := filepath.Abs(cfg.WebRoot)
	if err != nil {
		webRoot = cfg.WebRoot
	}
	fileServer := http.FileServer(http.Dir(webRoot))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		fileServer.ServeHTTP(w, r)
	})

	return &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           withCORS(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Admin-User-Id, X-Admin-Display-Name, X-CSRF-TOKEN")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func ListenAndServe(srv *http.Server) error {
	log.Printf("listening on %s", srv.Addr)
	return srv.ListenAndServe()
}
