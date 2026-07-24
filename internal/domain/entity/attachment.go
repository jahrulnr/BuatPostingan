package entity

import (
	"time"

	"buatpostingan/internal/domain/valueobject"
)

// AttachmentMeta describes one uploaded file bound to a thread.
type AttachmentMeta struct {
	ID           string
	ThreadID     valueobject.ThreadID
	Filename     string
	Mime         string
	Size         int64
	Kind         string // "image" | "text" | "binary"
	StoredName   string // basename under thread attachment dir
	Width        int    // image only; 0 if unknown
	Height       int    // image only; 0 if unknown
	UploadedAt   time.Time
	UploadedByID int64
}
