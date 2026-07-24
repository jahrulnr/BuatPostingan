package jsonl

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/enum"
	"buatpostingan/internal/domain/repository"
	"buatpostingan/internal/domain/valueobject"
	"buatpostingan/internal/pkg/apperr"
	"buatpostingan/internal/pkg/idgen"
)

// Store is the durable JSONL + session_index ThreadStore.
type Store struct {
	root string
}

var _ repository.ThreadStore = (*Store)(nil)

// NewStore creates a ThreadStore rooted at storageRoot (BP_STORAGE_ROOT).
func NewStore(storageRoot string) *Store {
	return &Store{root: storageRoot}
}

func (s *Store) CreateThread(ctx context.Context, createdByAdminUserID int64) (entity.ThreadSnapshot, error) {
	_ = ctx
	tid, err := valueobject.NewThreadID(idgen.ThreadID())
	if err != nil {
		return entity.ThreadSnapshot{}, err
	}
	now := time.Now().UTC()
	meta := entity.ConversationMeta{
		ThreadID:             tid,
		TitleSource:          enum.TitlePending,
		Status:               enum.ConversationActive,
		CreatedByAdminUserID: createdByAdminUserID,
		UpdatedAt:            now,
		LastActivityAt:       now,
	}
	if err := s.AppendConversationMeta(ctx, meta); err != nil {
		return entity.ThreadSnapshot{}, err
	}
	itemID, err := valueobject.NewItemID(idgen.ItemID())
	if err != nil {
		return entity.ThreadSnapshot{}, err
	}
	item, err := s.AppendItem(ctx, tid, entity.TranscriptItem{
		ID:       itemID,
		ThreadID: tid,
		Type:     enum.ItemThreadStarted,
		At:       now,
	})
	if err != nil {
		return entity.ThreadSnapshot{}, err
	}
	return entity.ThreadSnapshot{
		ThreadID: tid,
		SeqHead:  item.Seq,
		Items:    []entity.TranscriptItem{item},
	}, nil
}

func (s *Store) GetThread(ctx context.Context, threadID valueobject.ThreadID, afterSeq uint64) (entity.ThreadSnapshot, error) {
	meta, ok, err := s.ResolveConversation(ctx, threadID)
	if err != nil {
		return entity.ThreadSnapshot{}, err
	}
	if !ok || meta.Status == enum.ConversationDeleted {
		return entity.ThreadSnapshot{}, apperr.NotFound("thread not found")
	}
	lines, err := s.readThreadLines(threadID.String())
	if err != nil {
		return entity.ThreadSnapshot{}, err
	}
	items := make([]entity.TranscriptItem, 0, len(lines))
	var seqHead uint64
	for _, it := range lines {
		if it.Seq > seqHead {
			seqHead = it.Seq
		}
		if it.Seq > afterSeq {
			items = append(items, it)
		}
	}
	if head, herr := s.SeqHead(ctx, threadID); herr == nil && head > seqHead {
		seqHead = head
	}
	snap := entity.ThreadSnapshot{
		ThreadID:                   threadID,
		SeqHead:                    seqHead,
		FloorHolderAdminID:         meta.FloorHolderAdminID,
		ActiveTurnID:               meta.ActiveTurnID,
		ActiveTurnInitiatorAdminID: meta.ActiveTurnInitiatorAdminID,
		Items:                      items,
	}
	return snap, nil
}

func (s *Store) AppendItem(ctx context.Context, threadID valueobject.ThreadID, item entity.TranscriptItem) (entity.TranscriptItem, error) {
	_ = ctx
	if threadID.String() == "" || item.Type == "" || item.ID.String() == "" {
		return entity.TranscriptItem{}, apperr.Validation("thread_id, type, id required")
	}
	seq, err := s.nextSeq(threadID.String())
	if err != nil {
		return entity.TranscriptItem{}, err
	}
	item.Seq = seq
	item.ThreadID = threadID
	if item.At.IsZero() {
		item.At = time.Now().UTC()
	}
	raw := itemToLineMap(item)
	if err := appendJSONLLine(s.threadJSONLPath(threadID.String()), raw); err != nil {
		return entity.TranscriptItem{}, err
	}
	return item, nil
}

func (s *Store) ListConversations(ctx context.Context) ([]entity.ConversationMeta, error) {
	_ = ctx
	rows, err := readSessionIndexLines(s.sessionIndexPath())
	if err != nil {
		return nil, err
	}
	seen := make(map[string]entity.ConversationMeta, len(rows))
	for _, row := range rows {
		meta, err := rowToMeta(row)
		if err != nil {
			continue
		}
		seen[meta.ThreadID.String()] = meta
	}
	out := make([]entity.ConversationMeta, 0, len(seen))
	for _, meta := range seen {
		if meta.Status == enum.ConversationDeleted {
			continue
		}
		out = append(out, meta)
	}
	sort.Slice(out, func(i, j int) bool {
		ai := out[i].LastActivityAt
		if ai.IsZero() {
			ai = out[i].UpdatedAt
		}
		aj := out[j].LastActivityAt
		if aj.IsZero() {
			aj = out[j].UpdatedAt
		}
		return ai.After(aj)
	})
	return out, nil
}

func (s *Store) RenameThread(ctx context.Context, threadID valueobject.ThreadID, title valueobject.Title) error {
	prev, ok, err := s.ResolveConversation(ctx, threadID)
	if err != nil {
		return err
	}
	if !ok || prev.Status == enum.ConversationDeleted {
		return apperr.NotFound("thread not found")
	}
	now := time.Now().UTC()
	prev.Title = &title
	prev.TitleSource = enum.TitleManual
	prev.UpdatedAt = now
	return s.AppendConversationMeta(ctx, prev)
}

func (s *Store) SoftDeleteThread(ctx context.Context, threadID valueobject.ThreadID) error {
	prev, ok, err := s.ResolveConversation(ctx, threadID)
	if err != nil {
		return err
	}
	if !ok {
		return apperr.NotFound("thread not found")
	}
	// Reject if lock file exists (no TTL check — mirrors AIPedia).
	if _, err := os.Stat(s.threadLockPath(threadID.String())); err == nil {
		return apperr.ThreadBusy()
	} else if !os.IsNotExist(err) {
		return err
	}

	_ = os.Remove(s.threadJSONLPath(threadID.String()))
	_ = os.Remove(s.threadSeqPath(threadID.String()))
	interruptDir := s.interruptDir(threadID.String())
	if entries, err := os.ReadDir(interruptDir); err == nil {
		for _, e := range entries {
			_ = os.Remove(filepath.Join(interruptDir, e.Name()))
		}
		_ = os.Remove(interruptDir)
	}

	now := time.Now().UTC()
	prev.Status = enum.ConversationDeleted
	prev.UpdatedAt = now
	prev.FloorHolderAdminID = nil
	prev.FloorLastTurnAt = nil
	prev.ActiveTurnID = nil
	prev.ActiveTurnInitiatorAdminID = nil
	return s.AppendConversationMeta(ctx, prev)
}

func (s *Store) SeqHead(ctx context.Context, threadID valueobject.ThreadID) (uint64, error) {
	_ = ctx
	raw, err := os.ReadFile(s.threadSeqPath(threadID.String()))
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	sTrim := string(raw)
	for len(sTrim) > 0 && (sTrim[len(sTrim)-1] == '\n' || sTrim[len(sTrim)-1] == '\r' || sTrim[len(sTrim)-1] == ' ') {
		sTrim = sTrim[:len(sTrim)-1]
	}
	if sTrim == "" {
		return 0, nil
	}
	n, err := strconv.ParseUint(sTrim, 10, 64)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (s *Store) ResolveConversation(ctx context.Context, threadID valueobject.ThreadID) (entity.ConversationMeta, bool, error) {
	_ = ctx
	rows, err := readSessionIndexLines(s.sessionIndexPath())
	if err != nil {
		return entity.ConversationMeta{}, false, err
	}
	var found *entity.ConversationMeta
	for _, row := range rows {
		if row.ThreadID != threadID.String() {
			continue
		}
		meta, err := rowToMeta(row)
		if err != nil {
			continue
		}
		cp := meta
		found = &cp
	}
	if found == nil {
		return entity.ConversationMeta{}, false, nil
	}
	return *found, true, nil
}

func (s *Store) AppendConversationMeta(ctx context.Context, meta entity.ConversationMeta) error {
	_ = ctx
	if meta.ThreadID.String() == "" {
		return apperr.Validation("thread_id required")
	}
	now := time.Now().UTC()
	if meta.UpdatedAt.IsZero() {
		meta.UpdatedAt = now
	} else {
		meta.UpdatedAt = now // always bump updated_at on append (AIPedia)
	}
	if meta.LastActivityAt.IsZero() {
		meta.LastActivityAt = meta.UpdatedAt
	}
	if meta.Status == "" {
		meta.Status = enum.ConversationActive
	}
	if meta.TitleSource == "" {
		meta.TitleSource = enum.TitlePending
	}
	path := s.threadJSONLPath(meta.ThreadID.String())
	row := metaToRow(meta, path)
	return appendJSONLLine(s.sessionIndexPath(), row)
}

func (s *Store) ClearActiveTurn(ctx context.Context, threadID valueobject.ThreadID) error {
	prev, ok, err := s.ResolveConversation(ctx, threadID)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	now := time.Now().UTC()
	prev.UpdatedAt = now
	prev.LastActivityAt = now
	prev.ActiveTurnID = nil
	prev.ActiveTurnInitiatorAdminID = nil
	return s.AppendConversationMeta(ctx, prev)
}

func (s *Store) nextSeq(threadID string) (uint64, error) {
	path := s.threadSeqPath(threadID)
	var next uint64
	err := withFileLock(path, func(f *os.File) error {
		buf := make([]byte, 64)
		n, _ := f.ReadAt(buf, 0)
		last := uint64(0)
		if n > 0 {
			sTrim := string(buf[:n])
			for len(sTrim) > 0 && (sTrim[len(sTrim)-1] == '\n' || sTrim[len(sTrim)-1] == '\r' || sTrim[len(sTrim)-1] == ' ' || sTrim[len(sTrim)-1] == '\x00') {
				sTrim = sTrim[:len(sTrim)-1]
			}
			if sTrim != "" {
				v, err := strconv.ParseUint(sTrim, 10, 64)
				if err != nil {
					return err
				}
				last = v
			}
		}
		next = last + 1
		if err := f.Truncate(0); err != nil {
			return err
		}
		if _, err := f.Seek(0, 0); err != nil {
			return err
		}
		_, err := f.Write([]byte(strconv.FormatUint(next, 10)))
		return err
	})
	return next, err
}

func (s *Store) readThreadLines(threadID string) ([]entity.TranscriptItem, error) {
	path := s.threadJSONLPath(threadID)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var items []entity.TranscriptItem
	start := 0
	for i := 0; i <= len(raw); i++ {
		if i < len(raw) && raw[i] != '\n' {
			continue
		}
		line := raw[start:i]
		start = i + 1
		if len(line) == 0 {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(line, &m); err != nil {
			continue
		}
		it, err := lineMapToItem(m)
		if err != nil {
			continue
		}
		items = append(items, it)
	}
	return items, nil
}
