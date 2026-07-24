package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"buatpostingan/internal/domain/service"
)

const (
	defaultAttachmentMaxChars = 12000
	// MaxVisionImageBytes is the per-image cap for multimodal LLM payloads.
	MaxVisionImageBytes = 4 << 20 // 4 MiB
	// MaxVisionImagesPerMessage caps images injected into one user turn.
	MaxVisionImagesPerMessage = 4
	// MaxVisionImagesTotal caps images across the rebuilt message list.
	MaxVisionImagesTotal = 8
)

func (r *Registry) execReadAttachment(ctx context.Context, args map[string]any) service.ToolEnvelope {
	threadID, ok := threadIDFrom(ctx)
	if !ok {
		return service.ToolEnvelope{
			OK: false, Tool: "read_attachment",
			Error: map[string]any{"code": "missing_thread", "message": "attachment tools require an active thread"},
			Meta:  map[string]any{"truncated": false, "data_is_untrusted": true},
		}
	}
	if r.attachments == nil {
		return service.ToolEnvelope{
			OK: false, Tool: "read_attachment",
			Error: map[string]any{"code": "not_configured", "message": "attachment store not configured"},
			Meta:  map[string]any{"truncated": false, "data_is_untrusted": true},
		}
	}
	attID := strings.TrimSpace(asString(args["attachment_id"]))
	if attID == "" {
		return service.ToolEnvelope{
			OK: false, Tool: "read_attachment",
			Error: map[string]any{"code": "validation", "message": "attachment_id required"},
			Meta:  map[string]any{"truncated": false, "data_is_untrusted": true},
		}
	}
	meta, data, err := r.attachments.Get(ctx, threadID, attID)
	if err != nil {
		return service.ToolEnvelope{
			OK: false, Tool: "read_attachment",
			Error: map[string]any{"code": "not_found", "message": "attachment not found in this thread"},
			Meta:  map[string]any{"truncated": false, "data_is_untrusted": true},
		}
	}
	if meta.Kind != "text" {
		return service.ToolEnvelope{
			OK: false, Tool: "read_attachment",
			Error: map[string]any{
				"code":    "wrong_kind",
				"message": "use read_image for image attachments; this file kind is " + meta.Kind,
			},
			Data: map[string]any{
				"attachment_id": meta.ID,
				"filename":      meta.Filename,
				"mime":          meta.Mime,
				"kind":          meta.Kind,
				"size":          meta.Size,
			},
			Meta: map[string]any{"truncated": false, "data_is_untrusted": true},
		}
	}

	maxChars := asInt(args["max_chars"], defaultAttachmentMaxChars)
	if maxChars < 1 {
		maxChars = defaultAttachmentMaxChars
	}
	if maxChars > 20000 {
		maxChars = 20000
	}
	offset := asInt(args["offset"], 0)
	if offset < 0 {
		offset = 0
	}

	content := string(data)
	if !utf8.ValidString(content) {
		content = strings.ToValidUTF8(content, "\uFFFD")
	}
	runes := []rune(content)
	if offset > len(runes) {
		offset = len(runes)
	}
	end := offset + maxChars
	if end > len(runes) {
		end = len(runes)
	}
	slice := string(runes[offset:end])
	truncated := end < len(runes)
	var next any
	if truncated {
		next = end
	}

	relPath := filepath.Join("attachments", threadID.String(), meta.StoredName)
	return service.ToolEnvelope{
		OK:   true,
		Tool: "read_attachment",
		Data: map[string]any{
			"attachment_id": meta.ID,
			"filename":      meta.Filename,
			"mime":          meta.Mime,
			"kind":          meta.Kind,
			"size":          meta.Size,
			"path":          relPath,
			"content":       slice,
			"offset":        offset,
			"has_more":      truncated,
			"next_offset":   next,
			"total_chars":   len(runes),
		},
		Meta: map[string]any{
			"truncated":         truncated,
			"data_is_untrusted": true,
		},
	}
}

func (r *Registry) execReadImage(ctx context.Context, args map[string]any) service.ToolEnvelope {
	threadID, ok := threadIDFrom(ctx)
	if !ok {
		return service.ToolEnvelope{
			OK: false, Tool: "read_image",
			Error: map[string]any{"code": "missing_thread", "message": "attachment tools require an active thread"},
			Meta:  map[string]any{"truncated": false, "data_is_untrusted": true},
		}
	}
	if r.attachments == nil {
		return service.ToolEnvelope{
			OK: false, Tool: "read_image",
			Error: map[string]any{"code": "not_configured", "message": "attachment store not configured"},
			Meta:  map[string]any{"truncated": false, "data_is_untrusted": true},
		}
	}
	attID := strings.TrimSpace(asString(args["attachment_id"]))
	if attID == "" {
		return service.ToolEnvelope{
			OK: false, Tool: "read_image",
			Error: map[string]any{"code": "validation", "message": "attachment_id required"},
			Meta:  map[string]any{"truncated": false, "data_is_untrusted": true},
		}
	}
	meta, data, err := r.attachments.Get(ctx, threadID, attID)
	if err != nil {
		return service.ToolEnvelope{
			OK: false, Tool: "read_image",
			Error: map[string]any{"code": "not_found", "message": "attachment not found in this thread"},
			Meta:  map[string]any{"truncated": false, "data_is_untrusted": true},
		}
	}
	if meta.Kind != "image" {
		return service.ToolEnvelope{
			OK: false, Tool: "read_image",
			Error: map[string]any{
				"code":    "wrong_kind",
				"message": "use read_attachment for text files; this file kind is " + meta.Kind,
			},
			Data: map[string]any{
				"attachment_id": meta.ID,
				"filename":      meta.Filename,
				"mime":          meta.Mime,
				"kind":          meta.Kind,
				"size":          meta.Size,
			},
			Meta: map[string]any{"truncated": false, "data_is_untrusted": true},
		}
	}

	size := meta.Size
	if int64(len(data)) > 0 {
		size = int64(len(data))
	} else if _, path, pathErr := r.attachments.ResolvePath(ctx, threadID, attID); pathErr == nil {
		if st, stErr := os.Stat(path); stErr == nil && st != nil {
			size = st.Size()
		}
	}
	relPath := filepath.Join("attachments", threadID.String(), meta.StoredName)
	bytesOK := len(data) > 0 && int64(len(data)) <= MaxVisionImageBytes
	gateOK := true
	if r.vision != nil {
		gateOK = r.vision.AllowPixels(ctx)
	}
	visionOK := bytesOK && gateOK
	note := "Image pixels are included in the multimodal user message for vision-capable models when under the size limit. Use this tool to confirm metadata (filename, dimensions, attachment_id); do not claim you cannot see the image when it was attached to the turn."
	if len(data) == 0 {
		note = "Image bytes could not be loaded; vision is unavailable for this attachment."
	} else if int64(len(data)) > MaxVisionImageBytes {
		note = fmt.Sprintf("Image exceeds the %d-byte vision limit and was not sent as multimodal pixels; cite metadata only.", MaxVisionImageBytes)
	} else if !gateOK {
		note = "Vision pixels are disabled for this model/config (BP_LLM_VISION=off or auto with a text-only model); cite metadata only."
	}
	return service.ToolEnvelope{
		OK:   true,
		Tool: "read_image",
		Data: map[string]any{
			"attachment_id":             meta.ID,
			"filename":                  meta.Filename,
			"mime":                      meta.Mime,
			"kind":                      meta.Kind,
			"size":                      size,
			"width":                     meta.Width,
			"height":                    meta.Height,
			"path":                      relPath,
			"vision_available":          visionOK,
			"ocr_available":             false,
			"content_provided_to_model": visionOK,
			"note":                      note,
		},
		Meta: map[string]any{
			"truncated":         false,
			"data_is_untrusted": true,
		},
	}
}
