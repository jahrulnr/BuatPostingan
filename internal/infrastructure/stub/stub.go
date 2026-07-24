package stub

import (
	"context"

	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/repository"
	"buatpostingan/internal/domain/service"
	"buatpostingan/internal/domain/valueobject"
	"buatpostingan/internal/pkg/apperr"
)

// Package stub provides no-op / 501 adapters so cmd/app compiles
// before real JSONL / LLM / worker implementations land.

type ThreadStore struct{}

var _ repository.ThreadStore = (*ThreadStore)(nil)

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

type ThreadLock struct{}

var _ repository.ThreadLock = (*ThreadLock)(nil)

func (ThreadLock) TryAcquire(context.Context, valueobject.ThreadID) (func(), error) {
	return nil, apperr.NotImplemented("ThreadLock.TryAcquire")
}

type InterruptFlag struct{}

var _ repository.InterruptFlag = (*InterruptFlag)(nil)

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

var _ service.SpeakFloor = (*SpeakFloor)(nil)

func (SpeakFloor) Assert(context.Context, valueobject.ThreadID, int64) error {
	return apperr.NotImplemented("SpeakFloor.Assert")
}
func (SpeakFloor) Acquire(context.Context, valueobject.ThreadID, int64) error {
	return apperr.NotImplemented("SpeakFloor.Acquire")
}
func (SpeakFloor) Remaining(context.Context, valueobject.ThreadID) (*int64, int, error) {
	return nil, 0, apperr.NotImplemented("SpeakFloor.Remaining")
}

type TurnRateLimit struct{}

var _ service.TurnRateLimit = (*TurnRateLimit)(nil)

func (TurnRateLimit) Assert(context.Context, int64) (int, error) {
	return 0, apperr.NotImplemented("TurnRateLimit.Assert")
}

type DocsIndex struct{}

var _ service.DocsIndex = (*DocsIndex)(nil)

func (DocsIndex) Gate(context.Context) (entity.DocsIndexGate, error) {
	return entity.DocsIndexGate{}, apperr.NotImplemented("DocsIndex.Gate")
}
func (DocsIndex) Search(context.Context, string, int) (any, error) {
	return nil, apperr.NotImplemented("DocsIndex.Search")
}
func (DocsIndex) Reindex(context.Context) error {
	return apperr.NotImplemented("DocsIndex.Reindex")
}

type TurnWorker struct{}

var _ service.TurnWorker = (*TurnWorker)(nil)

func (TurnWorker) Enqueue(context.Context, service.TurnJob) error {
	return apperr.NotImplemented("TurnWorker.Enqueue")
}
