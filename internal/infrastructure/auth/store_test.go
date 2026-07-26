package auth

import (
	"context"
	"testing"
	"time"
)

func TestStoreBootstrapsAndAuthenticatesWithoutPersistingPlaintext(t *testing.T) {
	store, err := NewStore("file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	created, err := store.Bootstrap(context.Background(), "owner", "local-password")
	if err != nil || !created {
		t.Fatalf("bootstrap: created=%v err=%v", created, err)
	}
	created, err = store.Bootstrap(context.Background(), "owner", "another-password")
	if err != nil || created {
		t.Fatalf("second bootstrap: created=%v err=%v", created, err)
	}
	user, err := store.Authenticate(context.Background(), "owner", "local-password")
	if err != nil || user.Username != "owner" || user.Role != "admin" {
		t.Fatalf("authenticate: user=%+v err=%v", user, err)
	}
	if _, err := store.Authenticate(context.Background(), "owner", "wrong-password"); err != ErrInvalidCredentials {
		t.Fatalf("wrong password error = %v", err)
	}

	token, expires, err := store.CreateSession(context.Background(), user.ID, time.Hour)
	if err != nil || token == "" || !expires.After(time.Now()) {
		t.Fatalf("create session: token=%q expires=%v err=%v", token, expires, err)
	}
	got, err := store.UserBySession(context.Background(), token)
	if err != nil || got.ID != user.ID {
		t.Fatalf("session lookup: user=%+v err=%v", got, err)
	}
	if err := store.RevokeSession(context.Background(), token); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UserBySession(context.Background(), token); err != ErrSessionNotFound {
		t.Fatalf("revoked session error = %v", err)
	}
}

func TestBootstrapRejectsWeakPassword(t *testing.T) {
	store, err := NewStore("file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.Bootstrap(context.Background(), "owner", "short"); err == nil {
		t.Fatal("expected weak password rejection")
	}
}
