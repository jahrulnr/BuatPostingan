package httpdelivery

import (
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"buatpostingan/internal/usecase/webchat"
	"buatpostingan/internal/config"
)

// MountWebchatAPI registers /api/webchat routes only (no static FE).
// Use this when embedding the AI kit into another product mux.
func MountWebchatAPI(mux *http.ServeMux, uc webchat.Usecase) {
	NewWebchatHandler(uc).Register(mux)
}

// MountHealthz registers GET /healthz.
func MountHealthz(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
}

// MountStaticWeb serves files from webRoot at "/" (skips /api/).
func MountStaticWeb(mux *http.ServeMux, webRoot string) {
	abs, err := filepath.Abs(webRoot)
	if err != nil {
		abs = webRoot
	}
	fileServer := http.FileServer(http.Dir(abs))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

// NewServer wires product HTTP: API + healthz + static web/.
// Other hosts should call MountWebchatAPI (and optionally MountHealthz) on their own mux.
func NewServer(cfg config.Config, uc webchat.Usecase) *http.Server {
	mux := http.NewServeMux()
	MountWebchatAPI(mux, uc)
	MountHealthz(mux)
	MountStaticWeb(mux, cfg.WebRoot)

	return &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           withCORS(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Admin-User-Id, X-Admin-Display-Name, X-CSRF-TOKEN, Last-Event-ID")
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
