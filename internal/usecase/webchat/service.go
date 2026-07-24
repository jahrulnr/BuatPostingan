package webchat

import (
	"buatpostingan/internal/domain/repository"
	"buatpostingan/internal/domain/service"
)

// Deps are infrastructure ports injected into Service (AIPedia service collaborators).
// Concrete adapters live under internal/infrastructure; keep them as interfaces here.
type Deps struct {
	Threads   repository.ThreadStore
	Locks     repository.ThreadLock
	Interrupt repository.InterruptFlag
	Floor     service.SpeakFloor
	RateLimit service.TurnRateLimit
	Redactor  service.SecretRedactor
	Docs      service.DocsIndex
	Events    service.EventStreamer
	Worker    service.TurnWorker
}

// Service implements Usecase by orchestrating ports (mirrors AipediaWebchatController).
type Service struct {
	deps Deps
}

func NewService(deps Deps) *Service {
	return &Service{deps: deps}
}

var _ Usecase = (*Service)(nil)
