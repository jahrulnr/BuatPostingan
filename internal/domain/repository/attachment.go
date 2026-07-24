package repository

import (
	"context"

	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/valueobject"
)

// AttachmentStore persists thread-scoped uploads under storage/webchat/attachments.
type AttachmentStore interface {
	Save(ctx context.Context, in SaveAttachmentInput) (entity.AttachmentMeta, error)
	Get(ctx context.Context, threadID valueobject.ThreadID, attachmentID string) (entity.AttachmentMeta, []byte, error)
	List(ctx context.Context, threadID valueobject.ThreadID) ([]entity.AttachmentMeta, error)
	// ResolvePath returns meta + absolute path to bytes (thread attachment store).
	ResolvePath(ctx context.Context, threadID valueobject.ThreadID, attachmentID string) (entity.AttachmentMeta, string, error)
}

// SaveAttachmentInput is one multipart upload after validation.
type SaveAttachmentInput struct {
	ThreadID     valueobject.ThreadID
	Filename     string
	Mime         string
	Data         []byte
	UploadedByID int64
}
