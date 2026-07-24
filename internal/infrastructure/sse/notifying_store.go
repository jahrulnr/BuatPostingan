package sse

import (
	"context"

	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/repository"
	"buatpostingan/internal/domain/valueobject"
)

// NotifyingStore wraps ThreadStore and wakes the hub on every durable append.
type NotifyingStore struct {
	Inner repository.ThreadStore
	Hub   *Hub
}

var _ repository.ThreadStore = (*NotifyingStore)(nil)

func (n *NotifyingStore) CreateThread(ctx context.Context, createdByAdminUserID int64) (entity.ThreadSnapshot, error) {
	snap, err := n.Inner.CreateThread(ctx, createdByAdminUserID)
	if err == nil && n.Hub != nil {
		// Inner.CreateThread appends via the raw store, so notify here.
		n.Hub.Notify(snap.ThreadID)
	}
	return snap, err
}

func (n *NotifyingStore) GetThread(ctx context.Context, threadID valueobject.ThreadID, afterSeq uint64) (entity.ThreadSnapshot, error) {
	return n.Inner.GetThread(ctx, threadID, afterSeq)
}

func (n *NotifyingStore) AppendItem(ctx context.Context, threadID valueobject.ThreadID, item entity.TranscriptItem) (entity.TranscriptItem, error) {
	out, err := n.Inner.AppendItem(ctx, threadID, item)
	if err == nil && n.Hub != nil {
		n.Hub.Notify(threadID)
	}
	return out, err
}

func (n *NotifyingStore) ListConversations(ctx context.Context) ([]entity.ConversationMeta, error) {
	return n.Inner.ListConversations(ctx)
}

func (n *NotifyingStore) RenameThread(ctx context.Context, threadID valueobject.ThreadID, title valueobject.Title) error {
	return n.Inner.RenameThread(ctx, threadID, title)
}

func (n *NotifyingStore) SoftDeleteThread(ctx context.Context, threadID valueobject.ThreadID) error {
	return n.Inner.SoftDeleteThread(ctx, threadID)
}

func (n *NotifyingStore) SeqHead(ctx context.Context, threadID valueobject.ThreadID) (uint64, error) {
	return n.Inner.SeqHead(ctx, threadID)
}

func (n *NotifyingStore) ResolveConversation(ctx context.Context, threadID valueobject.ThreadID) (entity.ConversationMeta, bool, error) {
	return n.Inner.ResolveConversation(ctx, threadID)
}

func (n *NotifyingStore) AppendConversationMeta(ctx context.Context, meta entity.ConversationMeta) error {
	return n.Inner.AppendConversationMeta(ctx, meta)
}

func (n *NotifyingStore) ClearActiveTurn(ctx context.Context, threadID valueobject.ThreadID) error {
	return n.Inner.ClearActiveTurn(ctx, threadID)
}
