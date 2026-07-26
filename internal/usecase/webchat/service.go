package webchat

import (
	"buatpostingan/internal/domain/repository"
	"buatpostingan/internal/domain/service"
)

// Deps are infrastructure ports injected into Service (AIPedia service collaborators).
// Concrete adapters live under internal/infrastructure; keep them as interfaces here.
type Deps struct {
	Threads     repository.ThreadStore
	Locks       repository.ThreadLock
	Interrupt   repository.InterruptFlag
	Floor       service.SpeakFloor
	Redactor    service.SecretRedactor
	Docs        service.DocsIndex
	Events      service.EventStreamer
	Worker      service.TurnWorker
	Attachments repository.AttachmentStore
	Models      service.ModelCatalog
	Pages       service.PageWorkspaceManager
	// WorkspaceRoot is the default working directory surfaced to the workspace
	// picker when the request omits a path. Mirrors BP_WORKSPACE_ROOT.
	WorkspaceRoot string
}

// Service implements Usecase by orchestrating ports (mirrors AipediaWebchatController).
type Service struct {
	deps Deps
}

func NewService(deps Deps) *Service {
	return &Service{deps: deps}
}

var _ Usecase = (*Service)(nil)
