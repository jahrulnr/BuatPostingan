package webchat

import (
	"context"
	"strings"
	"time"

	"buatpostingan/internal/config"
	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/enum"
	"buatpostingan/internal/domain/repository"
	"buatpostingan/internal/domain/service"
	"buatpostingan/internal/domain/valueobject"
	"buatpostingan/internal/pkg/apperr"
	"buatpostingan/internal/pkg/idgen"
)

func (s *Service) ListConversations(ctx context.Context) (ListConversationsResult, error) {
	gate, err := s.deps.Docs.Gate(ctx)
	if err != nil {
		return ListConversationsResult{}, err
	}
	rows, err := s.deps.Threads.ListConversations(ctx)
	if err != nil {
		return ListConversationsResult{}, err
	}
	out := ListConversationsResult{
		Conversations: make([]ConversationView, 0, len(rows)),
		DocsIndex:     gate,
	}
	for _, meta := range rows {
		holder, remaining, ferr := s.deps.Floor.Remaining(ctx, meta.ThreadID)
		if ferr != nil {
			holder, remaining = nil, 0
		}
		view := ConversationView{Meta: meta, FloorRemainingSec: remaining}
		if remaining <= 0 {
			view.Meta.FloorHolderAdminID = nil
		} else if holder != nil {
			view.Meta.FloorHolderAdminID = holder
		}
		out.Conversations = append(out.Conversations, view)
	}
	return out, nil
}

func (s *Service) CreateThread(ctx context.Context, adminUserID int64) (CreateThreadResult, error) {
	if err := s.requireDocsReady(ctx); err != nil {
		return CreateThreadResult{}, err
	}
	snap, err := s.deps.Threads.CreateThread(ctx, adminUserID)
	if err != nil {
		return CreateThreadResult{}, err
	}
	return CreateThreadResult{
		ThreadID:             snap.ThreadID,
		SeqHead:              snap.SeqHead,
		CreatedByAdminUserID: adminUserID,
		CreatedAt:            time.Now().UTC(),
	}, nil
}

func (s *Service) GetThread(ctx context.Context, threadID valueobject.ThreadID, afterSeq uint64) (entity.ThreadSnapshot, error) {
	snap, err := s.deps.Threads.GetThread(ctx, threadID, afterSeq)
	if err != nil {
		return entity.ThreadSnapshot{}, err
	}
	busy, _ := s.deps.Locks.IsBusy(ctx, threadID)
	snap.Busy = busy
	holder, remaining, ferr := s.deps.Floor.Remaining(ctx, threadID)
	if ferr == nil {
		snap.FloorHolderAdminID = holder
		snap.FloorRemainingSec = remaining
	}
	return snap, nil
}

func (s *Service) RenameThread(ctx context.Context, threadID valueobject.ThreadID, title valueobject.Title) (RenameResult, error) {
	if _, err := s.deps.Threads.GetThread(ctx, threadID, 0); err != nil {
		return RenameResult{}, err
	}
	if err := s.deps.Threads.RenameThread(ctx, threadID, title); err != nil {
		return RenameResult{}, err
	}
	return RenameResult{ThreadID: threadID, Title: title}, nil
}

func (s *Service) DeleteThread(ctx context.Context, threadID valueobject.ThreadID) error {
	return s.deps.Threads.SoftDeleteThread(ctx, threadID)
}

func (s *Service) StartTurn(ctx context.Context, in StartTurnInput) (StartTurnResult, error) {
	// Order mirrors AipediaWebchatController::startTurn
	// DocsGate → validate → access → rate → floor.assert → redact → lock → floor.acquire → enqueue
	msg := strings.TrimSpace(in.Message)
	attIDs := uniqueNonEmpty(in.AttachmentIDs)
	if msg == "" && len(attIDs) == 0 {
		return StartTurnResult{}, apperr.Empty("message empty")
	}
	if msg == "" {
		msg = "(attached files)"
	}

	providerID, modelID, effort, err := s.resolveTurnOverrides(ctx, in.Model, in.Effort)
	if err != nil {
		return StartTurnResult{}, err
	}

	if err := s.requireDocsReady(ctx); err != nil {
		return StartTurnResult{}, err
	}
	if _, err := s.deps.Threads.GetThread(ctx, in.ThreadID, 0); err != nil {
		return StartTurnResult{}, err
	}
	if err := s.assertAttachments(ctx, in.ThreadID, attIDs); err != nil {
		return StartTurnResult{}, err
	}
	if err := s.assertRate(ctx, in.AdminUserID); err != nil {
		return StartTurnResult{}, err
	}
	if err := s.deps.Floor.Assert(ctx, in.ThreadID, in.AdminUserID); err != nil {
		return StartTurnResult{}, err
	}

	safe := msg
	if s.deps.Redactor != nil {
		redacted, err := s.deps.Redactor.Redact(ctx, msg)
		if err != nil {
			return StartTurnResult{}, err
		}
		safe = redacted
	}

	lockToken, err := s.deps.Locks.TryAcquire(ctx, in.ThreadID)
	if err != nil {
		return StartTurnResult{}, err
	}

	turnID, err := valueobject.NewTurnID(idgen.TurnID())
	if err != nil {
		_ = s.deps.Locks.Release(ctx, in.ThreadID, lockToken)
		return StartTurnResult{}, err
	}
	if err := s.deps.Floor.Acquire(ctx, in.ThreadID, in.AdminUserID, turnID); err != nil {
		_ = s.deps.Locks.Release(ctx, in.ThreadID, lockToken)
		return StartTurnResult{}, err
	}

	seqHead, err := s.deps.Threads.SeqHead(ctx, in.ThreadID)
	if err != nil {
		_ = s.deps.Locks.Release(ctx, in.ThreadID, lockToken)
		return StartTurnResult{}, err
	}

	job := service.TurnJob{
		ThreadID:      in.ThreadID,
		TurnID:        turnID,
		AdminUserID:   in.AdminUserID,
		AdminName:     in.AdminName,
		Message:       safe,
		AttachmentIDs: attIDs,
		IsRetry:       false,
		LockToken:     lockToken,
		ProviderID:    providerID,
		Model:         modelID,
		Effort:        effort,
		Workspace:     in.Workspace,
		UIPath:        in.UIPath,
	}
	if err := s.deps.Worker.Enqueue(ctx, job); err != nil {
		_ = s.deps.Locks.Release(ctx, in.ThreadID, lockToken)
		return StartTurnResult{}, err
	}

	holder := in.AdminUserID
	return StartTurnResult{
		ThreadID:           in.ThreadID,
		TurnID:             turnID,
		SeqHead:            seqHead,
		Status:             "queued",
		FloorHolderAdminID: &holder,
		FloorRemainingSec:  0,
	}, nil
}

func (s *Service) ListModels(ctx context.Context) (entity.ModelsCatalog, error) {
	if s.deps.Models == nil {
		return entity.ModelsCatalog{}, apperr.NotImplemented("ListModels")
	}
	return s.deps.Models.ListModels(ctx)
}

func (s *Service) resolveTurnOverrides(ctx context.Context, model, effortRaw string) (providerID, modelID, effort string, err error) {
	effort, ok := config.NormalizeEffortOverride(effortRaw)
	if !ok {
		return "", "", "", apperr.Validation("effort not allowed")
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return "", "", effort, nil
	}
	if s.deps.Models == nil {
		return "", "", "", apperr.Validation("model not allowed")
	}
	providerID, err = s.deps.Models.ResolveModel(ctx, model)
	if err != nil {
		return "", "", "", err
	}
	modelID = model
	return providerID, modelID, effort, nil
}

func (s *Service) UploadAttachment(ctx context.Context, in UploadAttachmentInput) (entity.AttachmentMeta, error) {
	if s.deps.Attachments == nil {
		return entity.AttachmentMeta{}, apperr.NotImplemented("attachments")
	}
	if _, err := s.deps.Threads.GetThread(ctx, in.ThreadID, 0); err != nil {
		return entity.AttachmentMeta{}, err
	}
	return s.deps.Attachments.Save(ctx, repository.SaveAttachmentInput{
		ThreadID:     in.ThreadID,
		Filename:     in.Filename,
		Mime:         in.Mime,
		Data:         in.Data,
		UploadedByID: in.AdminUserID,
	})
}

func (s *Service) ListAttachments(ctx context.Context, threadID valueobject.ThreadID) ([]entity.AttachmentMeta, error) {
	if s.deps.Attachments == nil {
		return nil, apperr.NotImplemented("attachments")
	}
	if _, err := s.deps.Threads.GetThread(ctx, threadID, 0); err != nil {
		return nil, err
	}
	return s.deps.Attachments.List(ctx, threadID)
}

func (s *Service) assertAttachments(ctx context.Context, threadID valueobject.ThreadID, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	if s.deps.Attachments == nil {
		return apperr.Validation("attachments not configured")
	}
	for _, id := range ids {
		if _, _, err := s.deps.Attachments.ResolvePath(ctx, threadID, id); err != nil {
			return apperr.Validation("unknown attachment_id: " + id)
		}
	}
	return nil
}

func uniqueNonEmpty(ids []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func (s *Service) RetryTurn(ctx context.Context, in RetryTurnInput) (StartTurnResult, error) {
	if err := s.requireDocsReady(ctx); err != nil {
		return StartTurnResult{}, err
	}

	providerID, modelID, effort, err := s.resolveTurnOverrides(ctx, in.Model, in.Effort)
	if err != nil {
		return StartTurnResult{}, err
	}

	snap, err := s.deps.Threads.GetThread(ctx, in.ThreadID, 0)
	if err != nil {
		return StartTurnResult{}, err
	}

	userMsg, okUser := findUserMessage(snap.Items, in.TurnID)
	if !okUser {
		return StartTurnResult{}, apperr.NotFound("turn user_message not found")
	}
	if !isRetryableFailed(snap.Items, in.TurnID) {
		return StartTurnResult{}, apperr.NotRetryable("turn not retryable")
	}
	initiator := asInt64(userMsg.Payload["admin_user_id"])
	if initiator != 0 && initiator != in.AdminUserID {
		return StartTurnResult{}, apperr.NotInitiator("only initiator can retry")
	}
	if err := s.assertRate(ctx, in.AdminUserID); err != nil {
		return StartTurnResult{}, err
	}
	if err := s.deps.Floor.Assert(ctx, in.ThreadID, in.AdminUserID); err != nil {
		return StartTurnResult{}, err
	}

	lockToken, err := s.deps.Locks.TryAcquire(ctx, in.ThreadID)
	if err != nil {
		return StartTurnResult{}, err
	}
	if err := s.deps.Floor.Acquire(ctx, in.ThreadID, in.AdminUserID, in.TurnID); err != nil {
		_ = s.deps.Locks.Release(ctx, in.ThreadID, lockToken)
		return StartTurnResult{}, err
	}

	text, _ := userMsg.Payload["text"].(string)
	adminName, _ := userMsg.Payload["admin_display_name"].(string)

	job := service.TurnJob{
		ThreadID:    in.ThreadID,
		TurnID:      in.TurnID,
		AdminUserID: in.AdminUserID,
		AdminName:   adminName,
		Message:     text,
		IsRetry:     true,
		LockToken:   lockToken,
		ProviderID:  providerID,
		Model:       modelID,
		Effort:      effort,
		Workspace:   in.Workspace,
		UIPath:      in.UIPath,
	}
	if err := s.deps.Worker.Enqueue(ctx, job); err != nil {
		_ = s.deps.Locks.Release(ctx, in.ThreadID, lockToken)
		return StartTurnResult{}, err
	}

	holder := in.AdminUserID
	return StartTurnResult{
		ThreadID:           in.ThreadID,
		TurnID:             in.TurnID,
		SeqHead:            snap.SeqHead,
		Status:             "queued",
		FloorHolderAdminID: &holder,
		FloorRemainingSec:  0,
	}, nil
}

func (s *Service) InterruptTurn(ctx context.Context, threadID valueobject.ThreadID, turnID valueobject.TurnID, adminUserID int64) error {
	snap, err := s.deps.Threads.GetThread(ctx, threadID, 0)
	if err != nil {
		return err
	}
	active := turnID
	if active.String() == "" && snap.ActiveTurnID != nil {
		active = *snap.ActiveTurnID
	}
	if active.String() == "" {
		return apperr.Validation("turn_id required")
	}
	if snap.ActiveTurnInitiatorAdminID != nil && *snap.ActiveTurnInitiatorAdminID != adminUserID {
		return apperr.NotInitiator("only initiator can interrupt")
	}
	return s.deps.Interrupt.Request(ctx, threadID, active)
}

func (s *Service) SubscribeEvents(ctx context.Context, threadID valueobject.ThreadID, afterSeq uint64, emit EventEmitter) error {
	if _, err := s.deps.Threads.GetThread(ctx, threadID, 0); err != nil {
		return err
	}
	return s.deps.Events.Subscribe(ctx, threadID, afterSeq, service.EventEmitFn(emit))
}

func (s *Service) requireDocsReady(ctx context.Context) error {
	gate, err := s.deps.Docs.Gate(ctx)
	if err != nil {
		return err
	}
	if !gate.Usable {
		return apperr.DocsIndexNotReady(map[string]any{
			"usable":         gate.Usable,
			"status":         gate.Status,
			"message":        gate.Message,
			"document_count": gate.DocumentCount,
		})
	}
	return nil
}

func (s *Service) assertRate(ctx context.Context, adminUserID int64) error {
	retryAfter, err := s.deps.RateLimit.Assert(ctx, adminUserID)
	if err == nil {
		return nil
	}
	if ae, ok := apperr.As(err); ok && ae.Code == apperr.CodeRateLimited && retryAfter > 0 {
		return apperr.RateLimited(retryAfter)
	}
	return err
}

func findUserMessage(items []entity.TranscriptItem, turnID valueobject.TurnID) (entity.TranscriptItem, bool) {
	for _, it := range items {
		if it.Type == enum.ItemUserMessage && it.TurnID == turnID {
			return it, true
		}
	}
	return entity.TranscriptItem{}, false
}

func isRetryableFailed(items []entity.TranscriptItem, turnID valueobject.TurnID) bool {
	var lastFailed *entity.TranscriptItem
	var completed bool
	for i := range items {
		it := &items[i]
		if it.TurnID != turnID {
			continue
		}
		switch it.Type {
		case enum.ItemTurnCompleted:
			completed = true
		case enum.ItemTurnFailed:
			lastFailed = it
			completed = false
		}
	}
	if completed || lastFailed == nil {
		return false
	}
	if errObj, ok := lastFailed.Payload["error"].(map[string]any); ok {
		if code, _ := errObj["code"].(string); code == "interrupted" {
			return false
		}
	}
	return true
}

func asInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	default:
		return 0
	}
}
