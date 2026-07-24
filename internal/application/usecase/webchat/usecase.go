package webchat

import (
	"context"

	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/repository"
	"buatpostingan/internal/domain/service"
	"buatpostingan/internal/domain/valueobject"
	"buatpostingan/internal/pkg/apperr"
)

// Usecase orchestrates webchat application flows.
// Concrete persistence / LLM / tools are injected via ports.
type Usecase struct {
	Threads   repository.ThreadStore
	Locks     repository.ThreadLock
	Interrupt repository.InterruptFlag
	Floor     service.SpeakFloor
	RateLimit service.TurnRateLimit
	Docs      service.DocsIndex
	Worker    service.TurnWorker
}

func New(
	threads repository.ThreadStore,
	locks repository.ThreadLock,
	interrupt repository.InterruptFlag,
	floor service.SpeakFloor,
	rate service.TurnRateLimit,
	docs service.DocsIndex,
	worker service.TurnWorker,
) *Usecase {
	return &Usecase{
		Threads:   threads,
		Locks:     locks,
		Interrupt: interrupt,
		Floor:     floor,
		RateLimit: rate,
		Docs:      docs,
		Worker:    worker,
	}
}

type ListConversationsResult struct {
	Conversations []entity.ConversationMeta
	DocsIndex     entity.DocsIndexGate
}

func (u *Usecase) ListConversations(ctx context.Context) (ListConversationsResult, error) {
	_ = ctx
	return ListConversationsResult{}, apperr.NotImplemented("ListConversations")
}

func (u *Usecase) CreateThread(ctx context.Context, adminUserID int64) (entity.ThreadSnapshot, error) {
	_ = ctx
	_ = adminUserID
	return entity.ThreadSnapshot{}, apperr.NotImplemented("CreateThread")
}

func (u *Usecase) GetThread(ctx context.Context, threadID valueobject.ThreadID, afterSeq uint64) (entity.ThreadSnapshot, error) {
	_ = ctx
	_ = threadID
	_ = afterSeq
	return entity.ThreadSnapshot{}, apperr.NotImplemented("GetThread")
}

func (u *Usecase) RenameThread(ctx context.Context, threadID valueobject.ThreadID, title valueobject.Title) error {
	_ = ctx
	_ = threadID
	_ = title
	return apperr.NotImplemented("RenameThread")
}

type StartTurnInput struct {
	ThreadID    valueobject.ThreadID
	Message     string
	AdminUserID int64
	AdminName   string
}

type StartTurnResult struct {
	ThreadID          valueobject.ThreadID
	TurnID            valueobject.TurnID
	SeqHead           uint64
	Status            string
	FloorHolderAdminID *int64
	FloorRemainingSec int
}

func (u *Usecase) StartTurn(ctx context.Context, in StartTurnInput) (StartTurnResult, error) {
	_ = ctx
	_ = in
	return StartTurnResult{}, apperr.NotImplemented("StartTurn")
}

func (u *Usecase) RetryTurn(ctx context.Context, threadID valueobject.ThreadID, turnID valueobject.TurnID, adminUserID int64) (StartTurnResult, error) {
	_ = ctx
	_ = threadID
	_ = turnID
	_ = adminUserID
	return StartTurnResult{}, apperr.NotImplemented("RetryTurn")
}

func (u *Usecase) InterruptTurn(ctx context.Context, threadID valueobject.ThreadID, turnID valueobject.TurnID, adminUserID int64) error {
	_ = ctx
	_ = threadID
	_ = turnID
	_ = adminUserID
	return apperr.NotImplemented("InterruptTurn")
}
