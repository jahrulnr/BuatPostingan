package httpdelivery_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	httpdelivery "buatpostingan/delivery/http"
	"buatpostingan/internal/infrastructure/auth"
)

func TestAuthLoginMeAndLogout(t *testing.T) {
	store, err := auth.NewStore("file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.Bootstrap(context.Background(), "owner", "local-password"); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	h := &httpdelivery.AuthHandler{Store: store, SessionTTL: time.Hour}
	h.Register(mux)
	protected := httpdelivery.RequireAuth(store, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	mux.Handle("GET /protected", protected)

	unauth := httptest.NewRecorder()
	mux.ServeHTTP(unauth, httptest.NewRequest(http.MethodGet, "/protected", nil))
	if unauth.Code != http.StatusUnauthorized {
		t.Fatalf("unauth status=%d body=%s", unauth.Code, unauth.Body.String())
	}

	login := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"username":"owner","password":"local-password"}`))
	login.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	mux.ServeHTTP(loginRec, login)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", loginRec.Code, loginRec.Body.String())
	}
	cookie := loginRec.Result().Cookies()[0]
	if cookie.Name != "bp_session" || !cookie.HttpOnly || cookie.Value == "" {
		t.Fatalf("session cookie=%+v", cookie)
	}

	me := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	me.AddCookie(cookie)
	meRec := httptest.NewRecorder()
	mux.ServeHTTP(meRec, me)
	if meRec.Code != http.StatusOK {
		t.Fatalf("me status=%d body=%s", meRec.Code, meRec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(meRec.Body.Bytes(), &body); err != nil || body["user"] == nil {
		t.Fatalf("me body=%s err=%v", meRec.Body.String(), err)
	}

	protectedReq := httptest.NewRequest(http.MethodGet, "/protected", nil)
	protectedReq.AddCookie(cookie)
	protectedRec := httptest.NewRecorder()
	mux.ServeHTTP(protectedRec, protectedReq)
	if protectedRec.Code != http.StatusNoContent {
		t.Fatalf("protected status=%d", protectedRec.Code)
	}

	logout := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logout.AddCookie(cookie)
	logoutRec := httptest.NewRecorder()
	mux.ServeHTTP(logoutRec, logout)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("logout status=%d", logoutRec.Code)
	}
	protectedAfter := httptest.NewRequest(http.MethodGet, "/protected", nil)
	protectedAfter.AddCookie(cookie)
	protectedAfterRec := httptest.NewRecorder()
	mux.ServeHTTP(protectedAfterRec, protectedAfter)
	if protectedAfterRec.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session status=%d", protectedAfterRec.Code)
	}
}
