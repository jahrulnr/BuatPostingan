package stub

import (
	"context"

	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/repository"
	"buatpostingan/internal/domain/service"
	"buatpostingan/internal/domain/valueobject"
	"buatpostingan/internal/pkg/apperr"
)

// Package stub provides 501 adapters for ports (kept for tests / partial wiring).

type ThreadStore struct{}

var _ repository.ThreadStore = ThreadStore{}

func (ThreadStore) CreateThread(context.Context, int64) (entity.ThreadSnapshot, error) {
	return entity.ThreadSnapshot{}, apperr.NotImplemented("ThreadStore.CreateThread")
}
func (ThreadStore) GetThread(context.Context, valueobject.ThreadID, uint64) (entity.ThreadSnapshot, error) {
	return entity.ThreadSnapshot{}, apperr.NotImplemented("ThreadStore.GetThread")
}
func (ThreadStore) AppendItem(context.Context, valueobject.ThreadID, entity.TranscriptItem) (entity.TranscriptItem, error) {
	return entity.TranscriptItem{}, apperr.NotImplemented("ThreadStore.AppendItem")
}
func (ThreadStore) ListConversations(context.Context) ([]entity.ConversationMeta, error) {
	return nil, apperr.NotImplemented("ThreadStore.ListConversations")
}
func (ThreadStore) RenameThread(context.Context, valueobject.ThreadID, valueobject.Title) error {
	return apperr.NotImplemented("ThreadStore.RenameThread")
}
func (ThreadStore) SoftDeleteThread(context.Context, valueobject.ThreadID) error {
	return apperr.NotImplemented("ThreadStore.SoftDeleteThread")
}
func (ThreadStore) SeqHead(context.Context, valueobject.ThreadID) (uint64, error) {
	return 0, apperr.NotImplemented("ThreadStore.SeqHead")
}
func (ThreadStore) ResolveConversation(context.Context, valueobject.ThreadID) (entity.ConversationMeta, bool, error) {
	return entity.ConversationMeta{}, false, apperr.NotImplemented("ThreadStore.ResolveConversation")
}
func (ThreadStore) AppendConversationMeta(context.Context, entity.ConversationMeta) error {
	return apperr.NotImplemented("ThreadStore.AppendConversationMeta")
}
func (ThreadStore) ClearActiveTurn(context.Context, valueobject.ThreadID) error {
	return apperr.NotImplemented("ThreadStore.ClearActiveTurn")
}

type ThreadLock struct{}

var _ repository.ThreadLock = ThreadLock{}

func (ThreadLock) TryAcquire(context.Context, valueobject.ThreadID) (string, error) {
	return "", apperr.NotImplemented("ThreadLock.TryAcquire")
}
func (ThreadLock) Release(context.Context, valueobject.ThreadID, string) error {
	return apperr.NotImplemented("ThreadLock.Release")
}
func (ThreadLock) IsBusy(context.Context, valueobject.ThreadID) (bool, error) {
	return false, apperr.NotImplemented("ThreadLock.IsBusy")
}

type InterruptFlag struct{}

var _ repository.InterruptFlag = InterruptFlag{}

func (InterruptFlag) Request(context.Context, valueobject.ThreadID, valueobject.TurnID) error {
	return apperr.NotImplemented("InterruptFlag.Request")
}
func (InterruptFlag) IsRequested(context.Context, valueobject.ThreadID, valueobject.TurnID) (bool, error) {
	return false, apperr.NotImplemented("InterruptFlag.IsRequested")
}
func (InterruptFlag) Clear(context.Context, valueobject.ThreadID, valueobject.TurnID) error {
	return apperr.NotImplemented("InterruptFlag.Clear")
}

type SpeakFloor struct{}

var _ service.SpeakFloor = SpeakFloor{}

func (SpeakFloor) Assert(context.Context, valueobject.ThreadID, int64) error {
	return apperr.NotImplemented("SpeakFloor.Assert")
}
func (SpeakFloor) Acquire(context.Context, valueobject.ThreadID, int64, valueobject.TurnID) error {
	return apperr.NotImplemented("SpeakFloor.Acquire")
}
func (SpeakFloor) Remaining(context.Context, valueobject.ThreadID) (*int64, int, error) {
	return nil, 0, nil // soft: list/get tolerate missing floor probe
}

type SecretRedactor struct{}

var _ service.SecretRedactor = SecretRedactor{}

func (SecretRedactor) Redact(_ context.Context, text string) (string, error) {
	return text, nil // identity until real scrubber lands
}

type DocsIndex struct{}

var _ service.DocsIndex = DocsIndex{}

func (DocsIndex) Gate(context.Context) (entity.DocsIndexGate, error) {
	return entity.DocsIndexGate{}, apperr.NotImplemented("DocsIndex.Gate")
}
func (DocsIndex) Search(context.Context, string, int) (any, error) {
	return nil, apperr.NotImplemented("DocsIndex.Search")
}
func (DocsIndex) Reindex(context.Context) error {
	return apperr.NotImplemented("DocsIndex.Reindex")
}

type EventStreamer struct{}

var _ service.EventStreamer = EventStreamer{}

func (EventStreamer) Subscribe(context.Context, valueobject.ThreadID, uint64, service.EventEmitFn) error {
	return apperr.NotImplemented("EventStreamer.Subscribe")
}

type TurnWorker struct{}

var _ service.TurnWorker = TurnWorker{}

func (TurnWorker) Enqueue(context.Context, service.TurnJob) error {
	return apperr.NotImplemented("TurnWorker.Enqueue")
}
