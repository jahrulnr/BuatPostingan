package auth

import (
	"context"
	"os"
	"path/filepath"
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

func TestCreateSessionPurgesExpiredAndCapsPerUser(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "users.sqlite")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := st.Mode().Perm(); perm&0o077 != 0 {
		t.Fatalf("expected 0600-ish auth db perms, got %v", perm)
	}

	if _, err := store.Bootstrap(context.Background(), "owner", "local-password"); err != nil {
		t.Fatal(err)
	}
	user, err := store.Authenticate(context.Background(), "owner", "local-password")
	if err != nil {
		t.Fatal(err)
	}

	expired, _, err := store.CreateSession(context.Background(), user.ID, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, err := store.UserBySession(context.Background(), expired); err != ErrSessionNotFound {
		t.Fatalf("expired session error = %v", err)
	}

	for i := 0; i < MaxSessionsPerUser+3; i++ {
		if _, _, err := store.CreateSession(context.Background(), user.ID, time.Hour); err != nil {
			t.Fatalf("create session %d: %v", i, err)
		}
	}
	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE user_id = ?`, user.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != MaxSessionsPerUser {
		t.Fatalf("session cap: got %d want %d", count, MaxSessionsPerUser)
	}
}
