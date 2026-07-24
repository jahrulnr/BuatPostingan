package jsonl

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"time"

	"buatpostingan/internal/domain/repository"
	"buatpostingan/internal/domain/valueobject"
	"buatpostingan/internal/pkg/apperr"
)

const defaultLockTTLSec = 300

// Lock implements repository.ThreadLock with token + TTL.
// Hardened vs AIPedia PHP: refuses to overwrite an unexpired lock.
type Lock struct {
	root   string
	ttlSec int
}

var _ repository.ThreadLock = (*Lock)(nil)

func NewLock(storageRoot string, ttlSec int) *Lock {
	if ttlSec <= 0 {
		ttlSec = defaultLockTTLSec
	}
	return &Lock{root: storageRoot, ttlSec: ttlSec}
}

type lockRecord struct {
	ThreadID   string `json:"thread_id"`
	Token      string `json:"token"`
	ExpiresAt  int64  `json:"expires_at"`
	AcquiredAt int64  `json:"acquired_at"`
}

func (l *Lock) lockPath(threadID string) string {
	return (&Store{root: l.root}).threadLockPath(threadID)
}

func (l *Lock) TryAcquire(ctx context.Context, threadID valueobject.ThreadID) (string, error) {
	_ = ctx
	path := l.lockPath(threadID.String())
	var token string
	err := withFileLock(path, func(f *os.File) error {
		existing, ok := readLockRecord(f)
		now := time.Now().Unix()
		if ok && existing.ExpiresAt > now {
			return apperr.ThreadBusy()
		}
		token = newLockToken()
		rec := lockRecord{
			ThreadID:   threadID.String(),
			Token:      token,
			ExpiresAt:  now + int64(l.ttlSec),
			AcquiredAt: now,
		}
		if err := f.Truncate(0); err != nil {
			return err
		}
		if _, err := f.Seek(0, 0); err != nil {
			return err
		}
		enc := json.NewEncoder(f)
		enc.SetEscapeHTML(false)
		return enc.Encode(rec)
	})
	if err != nil {
		return "", err
	}
	return token, nil
}

func (l *Lock) Release(ctx context.Context, threadID valueobject.ThreadID, lockToken string) error {
	_ = ctx
	path := l.lockPath(threadID.String())
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(raw) == 0 {
		return nil
	}
	var rec lockRecord
	if err := json.Unmarshal(raw, &rec); err != nil || rec.Token != lockToken {
		return nil // mismatch: leave lock (log-equivalent no-op)
	}
	_ = os.Remove(path)
	return nil
}

func (l *Lock) IsBusy(ctx context.Context, threadID valueobject.ThreadID) (bool, error) {
	_ = ctx
	path := l.lockPath(threadID.String())
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if len(raw) == 0 {
		return false, nil
	}
	var rec lockRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return false, nil
	}
	if time.Now().Unix() > rec.ExpiresAt {
		_ = os.Remove(path)
		return false, nil
	}
	return true, nil
}

func readLockRecord(f *os.File) (lockRecord, bool) {
	buf := make([]byte, 4096)
	n, _ := f.ReadAt(buf, 0)
	if n == 0 {
		return lockRecord{}, false
	}
	var rec lockRecord
	if err := json.Unmarshal(buf[:n], &rec); err != nil || rec.Token == "" {
		return lockRecord{}, false
	}
	return rec, true
}

func newLockToken() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "ulid_" + hex.EncodeToString(b[:]) + hex.EncodeToString([]byte(time.Now().UTC().Format("150405.000")))
}
