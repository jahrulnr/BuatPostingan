package worker

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"buatpostingan/internal/domain/repository"
	"buatpostingan/internal/domain/valueobject"
	"buatpostingan/internal/infrastructure/service/tools"
)

// visionGate decides whether image pixels may be attached to LLM messages.
type visionGate interface {
	AllowPixels(ctx context.Context) bool
}

// visionLoader loads image attachment bytes for multimodal LLM parts.
type visionLoader struct {
	ctx      context.Context
	threadID valueobject.ThreadID
	store    repository.AttachmentStore
	// totalImages counts images already injected across the message list.
	totalImages int
	// skipPixels forces metadata-only (BP_LLM_VISION=off or auto+text-only model).
	skipPixels bool
}

func (v *visionLoader) load(attachmentID string) (mime string, data []byte, err error) {
	if v == nil || v.store == nil {
		return "", nil, fmt.Errorf("no attachment store")
	}
	meta, bytes, err := v.store.Get(v.ctx, v.threadID, attachmentID)
	if err != nil {
		return "", nil, err
	}
	if meta.Kind != "image" {
		return "", nil, fmt.Errorf("not an image")
	}
	mime = strings.TrimSpace(meta.Mime)
	if mime == "" {
		mime = "image/png"
	}
	return mime, bytes, nil
}

// userLLMContent builds chat-completions-shaped user content: plain string when
// there are no includable images, otherwise a content parts array with text +
// image_url data URLs (from attachment store only — never remote fetch).
func userLLMContent(payload map[string]any, loader *visionLoader) any {
	textPart := userContentFromPayload(payload)
	if loader == nil || loader.store == nil {
		return textPart
	}

	refs := llmAttachmentRefs(payload["attachments"])
	if len(refs) == 0 {
		return textPart
	}

	if loader.skipPixels {
		var notes []string
		for _, ref := range refs {
			kind, _ := ref["kind"].(string)
			if kind != "image" {
				continue
			}
			id, _ := ref["attachment_id"].(string)
			if id == "" {
				continue
			}
			notes = append(notes, fmt.Sprintf("[image %s skipped: vision pixels disabled for this model/config]", id))
		}
		if len(notes) == 0 {
			return textPart
		}
		extra := strings.Join(notes, "\n")
		if strings.TrimSpace(textPart) == "" {
			return extra
		}
		return textPart + "\n\n" + extra
	}

	var parts []map[string]any
	var notes []string
	imagesInMsg := 0

	for _, ref := range refs {
		kind, _ := ref["kind"].(string)
		if kind != "image" {
			continue
		}
		id, _ := ref["attachment_id"].(string)
		if id == "" {
			continue
		}
		if imagesInMsg >= tools.MaxVisionImagesPerMessage {
			notes = append(notes, fmt.Sprintf("[image %s skipped: max %d images per message]", id, tools.MaxVisionImagesPerMessage))
			continue
		}
		if loader.totalImages >= tools.MaxVisionImagesTotal {
			notes = append(notes, fmt.Sprintf("[image %s skipped: max %d images in context]", id, tools.MaxVisionImagesTotal))
			continue
		}
		mime, data, err := loader.load(id)
		if err != nil {
			notes = append(notes, fmt.Sprintf("[image %s skipped: unavailable]", id))
			continue
		}
		if int64(len(data)) > tools.MaxVisionImageBytes {
			notes = append(notes, fmt.Sprintf("[image %s skipped: exceeds %d byte vision limit]", id, tools.MaxVisionImageBytes))
			continue
		}
		if len(data) == 0 {
			notes = append(notes, fmt.Sprintf("[image %s skipped: empty]", id))
			continue
		}
		// Always data URLs from our store — never remote http(s) URLs (SSRF).
		dataURL := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
		parts = append(parts, map[string]any{
			"type": "image_url",
			"image_url": map[string]any{
				"url": dataURL,
			},
		})
		imagesInMsg++
		loader.totalImages++
	}

	if len(parts) == 0 {
		if len(notes) == 0 {
			return textPart
		}
		extra := strings.Join(notes, "\n")
		if strings.TrimSpace(textPart) == "" {
			return extra
		}
		return textPart + "\n\n" + extra
	}

	text := textPart
	if len(notes) > 0 {
		if strings.TrimSpace(text) == "" {
			text = strings.Join(notes, "\n")
		} else {
			text = text + "\n\n" + strings.Join(notes, "\n")
		}
	}
	out := make([]map[string]any, 0, 1+len(parts))
	if strings.TrimSpace(text) != "" {
		out = append(out, map[string]any{"type": "text", "text": text})
	} else {
		out = append(out, map[string]any{"type": "text", "text": "(image attachment)"})
	}
	out = append(out, parts...)
	return out
}
