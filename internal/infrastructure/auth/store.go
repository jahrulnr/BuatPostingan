package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrSessionNotFound    = errors.New("session not found")
	ErrNoUsers            = errors.New("no users configured")
)

// MaxSessionsPerUser caps concurrent sessions; oldest rows are pruned on create.
const MaxSessionsPerUser = 10

type User struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
}

type Store struct {
	db   *sql.DB
	path string
}

func NewStore(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("auth database path is required")
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open auth database: %w", err)
	}
	store := &Store{db: db, path: path}
	if err := store.init(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.hardenFilePerms(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) hardenFilePerms() error {
	if s.path == "" || strings.Contains(s.path, ":memory:") || strings.HasPrefix(s.path, "file:") {
		return nil
	}
	if err := os.Chmod(s.path, 0o600); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("chmod auth database: %w", err)
	}
	return nil
}

func (s *Store) init(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		PRAGMA busy_timeout = 5000;
		PRAGMA foreign_keys = ON;
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL COLLATE NOCASE UNIQUE,
			display_name TEXT NOT NULL,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'admin',
			active INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS sessions (
			token_hash TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_sessions_expiry ON sessions(expires_at);
		CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id, created_at);
	`)
	if err != nil {
		return fmt.Errorf("initialize auth database: %w", err)
	}
	return nil
}

func (s *Store) Bootstrap(ctx context.Context, username, password string) (bool, error) {
	username = strings.TrimSpace(username)
	if username == "" || len(username) > 64 {
		return false, errors.New("auth bootstrap username must be 1-64 characters")
	}
	if len(password) < 8 || len(password) > 128 {
		return false, errors.New("auth bootstrap password must be 8-128 characters")
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return false, fmt.Errorf("count auth users: %w", err)
	}
	if count > 0 {
		return false, nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return false, fmt.Errorf("hash bootstrap password: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(ctx, `INSERT INTO users(username, display_name, password_hash, role, created_at) VALUES (?, ?, ?, 'admin', ?)`, username, username, string(hash), now)
	if err != nil {
		return false, fmt.Errorf("create bootstrap user: %w", err)
	}
	return true, nil
}

func (s *Store) Authenticate(ctx context.Context, username, password string) (User, error) {
	var user User
	var hash string
	var active int
	err := s.db.QueryRowContext(ctx, `SELECT id, username, display_name, role, password_hash, active FROM users WHERE username = ?`, strings.TrimSpace(username)).Scan(&user.ID, &user.Username, &user.DisplayName, &user.Role, &hash, &active)
	if err != nil || active == 0 || bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return User{}, ErrInvalidCredentials
	}
	return user, nil
}

func (s *Store) CreateSession(ctx context.Context, userID int64, ttl time.Duration) (string, time.Time, error) {
	if userID <= 0 || ttl <= 0 {
		return "", time.Time{}, errors.New("invalid session parameters")
	}
	_ = s.PurgeExpiredSessions(ctx)

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, fmt.Errorf("generate session token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	expires := time.Now().UTC().Add(ttl)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `INSERT INTO sessions(token_hash, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`, base64.RawURLEncoding.EncodeToString(hash[:]), userID, expires.Format(time.RFC3339Nano), now)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("create session: %w", err)
	}
	if err := s.trimUserSessions(ctx, userID); err != nil {
		return "", time.Time{}, err
	}
	return token, expires, nil
}

func (s *Store) trimUserSessions(ctx context.Context, userID int64) error {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE user_id = ?`, userID).Scan(&count); err != nil {
		return fmt.Errorf("count user sessions: %w", err)
	}
	if count <= MaxSessionsPerUser {
		return nil
	}
	excess := count - MaxSessionsPerUser
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM sessions WHERE token_hash IN (
			SELECT token_hash FROM sessions
			WHERE user_id = ?
			ORDER BY created_at ASC
			LIMIT ?
		)
	`, userID, excess)
	if err != nil {
		return fmt.Errorf("trim user sessions: %w", err)
	}
	return nil
}

// PurgeExpiredSessions deletes sessions past expires_at.
func (s *Store) PurgeExpiredSessions(ctx context.Context) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, now)
	return err
}

func (s *Store) UserBySession(ctx context.Context, token string) (User, error) {
	hash := sha256.Sum256([]byte(token))
	var user User
	var expires string
	err := s.db.QueryRowContext(ctx, `SELECT u.id, u.username, u.display_name, u.role, s.expires_at FROM sessions s JOIN users u ON u.id = s.user_id WHERE s.token_hash = ? AND u.active = 1`, base64.RawURLEncoding.EncodeToString(hash[:])).Scan(&user.ID, &user.Username, &user.DisplayName, &user.Role, &expires)
	if err != nil {
		return User{}, ErrSessionNotFound
	}
	when, err := time.Parse(time.RFC3339Nano, expires)
	if err != nil || !when.After(time.Now().UTC()) {
		_, _ = s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, base64.RawURLEncoding.EncodeToString(hash[:]))
		return User{}, ErrSessionNotFound
	}
	return user, nil
}

func (s *Store) RevokeSession(ctx context.Context, token string) error {
	hash := sha256.Sum256([]byte(token))
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, base64.RawURLEncoding.EncodeToString(hash[:]))
	return err
}

func (s *Store) UserCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}
