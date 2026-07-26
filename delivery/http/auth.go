package httpdelivery

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"buatpostingan/delivery/presenter"
	"buatpostingan/internal/infrastructure/auth"
	"buatpostingan/internal/infrastructure/network"
	"buatpostingan/internal/pkg/apperr"
)

const sessionCookieName = "bp_session"

type authContextKey struct{}

func userFromContext(ctx context.Context) (auth.User, bool) {
	user, ok := ctx.Value(authContextKey{}).(auth.User)
	return user, ok
}

type AuthHandler struct {
	Store      *auth.Store
	SessionTTL time.Duration
	Limiter    *network.LoginLimiter
}

func (h *AuthHandler) limiter() *network.LoginLimiter {
	if h.Limiter != nil {
		return h.Limiter
	}
	return defaultLoginLimiter
}

var defaultLoginLimiter = network.NewLoginLimiter()

func (h *AuthHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/login", h.Login)
	mux.HandleFunc("POST /api/auth/logout", h.Logout)
	mux.HandleFunc("GET /api/auth/me", h.Me)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeErr(w, r, "auth.login", err)
		return
	}
	username := strings.TrimSpace(input.Username)
	if username == "" || len(input.Password) == 0 || len(input.Password) > 128 {
		presenter.WriteAppError(w, apperr.New(http.StatusUnprocessableEntity, apperr.CodeValidation, "username and password are required"))
		return
	}

	ip := network.ClientIP(r)
	lim := h.limiter()
	if ok, retryAfter := lim.Allow(ip, username); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
		presenter.WriteAppError(w, apperr.RateLimited("too many login attempts — try again later"))
		return
	}

	user, err := h.Store.Authenticate(r.Context(), username, input.Password)
	if err != nil {
		lim.RecordFailure(ip, username)
		presenter.WriteAppError(w, apperr.New(http.StatusUnauthorized, apperr.CodeUnauthorized, "invalid username or password"))
		return
	}
	lim.ClearUser(username)

	token, expires, err := h.Store.CreateSession(r.Context(), user.ID, h.SessionTTL)
	if err != nil {
		writeErr(w, r, "auth.create_session", err)
		return
	}
	h.setSessionCookie(w, r, token, expires)
	presenter.WriteJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil && cookie.Value != "" {
		_ = h.Store.RevokeSession(r.Context(), cookie.Value)
	}
	h.clearSessionCookie(w, r)
	presenter.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := h.userFromRequest(r)
	if !ok {
		presenter.WriteAppError(w, apperr.New(http.StatusUnauthorized, apperr.CodeUnauthorized, "authentication required"))
		return
	}
	presenter.WriteJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (h *AuthHandler) userFromRequest(r *http.Request) (auth.User, bool) {
	if user, ok := userFromContext(r.Context()); ok {
		return user, true
	}
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return auth.User{}, false
	}
	user, err := h.Store.UserBySession(r.Context(), cookie.Value)
	return user, err == nil
}

func (h *AuthHandler) setSessionCookie(w http.ResponseWriter, r *http.Request, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   maxAge(expires),
		HttpOnly: true,
		Secure:   requestIsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *AuthHandler) clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
		HttpOnly: true,
		Secure:   requestIsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func maxAge(expires time.Time) int {
	seconds := int(time.Until(expires).Seconds())
	if seconds < 1 {
		return 1
	}
	return seconds
}

func requestIsHTTPS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]), "https")
}

func RequireAuth(store *auth.Store, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || cookie.Value == "" {
			writeUnauthorized(w)
			return
		}
		user, err := store.UserBySession(r.Context(), cookie.Value)
		if err != nil {
			writeUnauthorized(w)
			return
		}
		ctx := context.WithValue(r.Context(), authContextKey{}, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func writeUnauthorized(w http.ResponseWriter) {
	presenter.WriteAppError(w, apperr.New(http.StatusUnauthorized, apperr.CodeUnauthorized, "authentication required"))
}

func shouldRequireAuth(path string) bool {
	return path == "/api/settings" || strings.HasPrefix(path, "/api/settings/") ||
		path == "/api/pages" || strings.HasPrefix(path, "/api/pages/") ||
		path == "/api/webchat" || strings.HasPrefix(path, "/api/webchat/")
}

func authGuard(store *auth.Store, next http.Handler) http.Handler {
	if store == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if shouldRequireAuth(r.URL.Path) {
			RequireAuth(store, next).ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
