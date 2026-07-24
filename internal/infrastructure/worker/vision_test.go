package worker

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/enum"
	"buatpostingan/internal/domain/repository"
	"buatpostingan/internal/domain/valueobject"
	"buatpostingan/internal/infrastructure/service/tools"
)

type memAttachments struct {
	byID map[string]struct {
		meta entity.AttachmentMeta
		data []byte
	}
}

func (m *memAttachments) Save(context.Context, repository.SaveAttachmentInput) (entity.AttachmentMeta, error) {
	return entity.AttachmentMeta{}, errors.New("unused")
}
func (m *memAttachments) List(context.Context, valueobject.ThreadID) ([]entity.AttachmentMeta, error) {
	return nil, nil
}
func (m *memAttachments) ResolvePath(context.Context, valueobject.ThreadID, string) (entity.AttachmentMeta, string, error) {
	return entity.AttachmentMeta{}, "", errors.New("unused")
}
func (m *memAttachments) Get(_ context.Context, _ valueobject.ThreadID, id string) (entity.AttachmentMeta, []byte, error) {
	row, ok := m.byID[id]
	if !ok {
		return entity.AttachmentMeta{}, nil, errors.New("not found")
	}
	return row.meta, row.data, nil
}

func tinyPNG() []byte {
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xde, 0x00, 0x00, 0x00,
		0x0c, 0x49, 0x44, 0x41, 0x54, 0x08, 0xd7, 0x63, 0xf8, 0xff, 0xff, 0x3f,
		0x00, 0x05, 0xfe, 0x02, 0xfe, 0xdc, 0xcc, 0x59, 0xe7, 0x00, 0x00, 0x00,
		0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}
}

func TestUserLLMContentIncludesImageURL(t *testing.T) {
	tid, _ := valueobject.NewThreadID("thr_vis1")
	png := tinyPNG()
	store := &memAttachments{byID: map[string]struct {
		meta entity.AttachmentMeta
		data []byte
	}{
		"att_img": {
			meta: entity.AttachmentMeta{ID: "att_img", Kind: "image", Mime: "image/png", Size: int64(len(png))},
			data: png,
		},
	}}
	loader := &visionLoader{ctx: context.Background(), threadID: tid, store: store}
	payload := map[string]any{
		"text": "apa isi gambar ini?",
		"attachments": []any{
			map[string]any{
				"attachment_id": "att_img",
				"filename":      "dot.png",
				"mime":          "image/png",
				"kind":          "image",
				"size":          len(png),
			},
		},
	}
	content := userLLMContent(payload, loader)
	parts, ok := content.([]map[string]any)
	if !ok || len(parts) < 2 {
		t.Fatalf("want multimodal parts, got %#v", content)
	}
	if parts[0]["type"] != "text" {
		t.Fatalf("first part %#v", parts[0])
	}
	text, _ := parts[0]["text"].(string)
	if !strings.Contains(text, "apa isi gambar") || !strings.Contains(text, "attachment_id") {
		t.Fatalf("text part missing metadata: %q", text)
	}
	if parts[1]["type"] != "image_url" {
		t.Fatalf("second part %#v", parts[1])
	}
	img, _ := parts[1]["image_url"].(map[string]any)
	url, _ := img["url"].(string)
	wantPrefix := "data:image/png;base64,"
	if !strings.HasPrefix(url, wantPrefix) {
		t.Fatalf("data url prefix: %q", url[:min(40, len(url))])
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(url, wantPrefix))
	if err != nil || !bytes.Equal(raw, png) {
		t.Fatalf("round-trip bytes err=%v len=%d", err, len(raw))
	}
}

func TestUserLLMContentRejectsOversizedImage(t *testing.T) {
	tid, _ := valueobject.NewThreadID("thr_vis2")
	big := bytes.Repeat([]byte{0xab}, int(tools.MaxVisionImageBytes)+1)
	store := &memAttachments{byID: map[string]struct {
		meta entity.AttachmentMeta
		data []byte
	}{
		"att_big": {
			meta: entity.AttachmentMeta{ID: "att_big", Kind: "image", Mime: "image/jpeg", Size: int64(len(big))},
			data: big,
		},
	}}
	loader := &visionLoader{ctx: context.Background(), threadID: tid, store: store}
	content := userLLMContent(map[string]any{
		"text": "lihat",
		"attachments": []any{
			map[string]any{"attachment_id": "att_big", "kind": "image", "mime": "image/jpeg"},
		},
	}, loader)
	s, ok := content.(string)
	if !ok {
		t.Fatalf("oversized should stay text, got %#v", content)
	}
	if !strings.Contains(s, "skipped") || !strings.Contains(s, "vision limit") {
		t.Fatalf("missing skip note: %q", s)
	}
	if loader.totalImages != 0 {
		t.Fatalf("totalImages=%d", loader.totalImages)
	}
}

func TestBuildMessagesVisionViaWorker(t *testing.T) {
	tid, _ := valueobject.NewThreadID("thr_vis3")
	turn, _ := valueobject.NewTurnID("trn_vis3")
	png := tinyPNG()
	store := &memAttachments{byID: map[string]struct {
		meta entity.AttachmentMeta
		data []byte
	}{
		"att_1": {
			meta: entity.AttachmentMeta{ID: "att_1", Kind: "image", Mime: "image/png"},
			data: png,
		},
	}}
	w := New(Deps{Attachments: store})
	items := []entity.TranscriptItem{{
		Type: enum.ItemUserMessage, ThreadID: tid, TurnID: turn,
		Payload: map[string]any{
			"text": "desc",
			"attachments": []any{
				map[string]any{"attachment_id": "att_1", "kind": "image", "mime": "image/png"},
			},
		},
	}}
	msgs := w.buildMessages(context.Background(), tid, items)
	if len(msgs) != 1 {
		t.Fatalf("len=%d", len(msgs))
	}
	parts, ok := msgs[0]["content"].([]map[string]any)
	if !ok || len(parts) < 2 || parts[1]["type"] != "image_url" {
		t.Fatalf("content=%#v", msgs[0]["content"])
	}
}

func TestUserLLMContentSkipPixels(t *testing.T) {
	tid, _ := valueobject.NewThreadID("thr_vis_off")
	png := tinyPNG()
	store := &memAttachments{byID: map[string]struct {
		meta entity.AttachmentMeta
		data []byte
	}{
		"att_img": {
			meta: entity.AttachmentMeta{ID: "att_img", Kind: "image", Mime: "image/png", Size: int64(len(png))},
			data: png,
		},
	}}
	loader := &visionLoader{ctx: context.Background(), threadID: tid, store: store, skipPixels: true}
	content := userLLMContent(map[string]any{
		"text": "lihat",
		"attachments": []any{
			map[string]any{"attachment_id": "att_img", "kind": "image", "mime": "image/png"},
		},
	}, loader)
	s, ok := content.(string)
	if !ok {
		t.Fatalf("want text-only, got %#v", content)
	}
	if !strings.Contains(s, "skipped") || !strings.Contains(s, "vision pixels disabled") {
		t.Fatalf("%q", s)
	}
}

type denyVision struct{}

func (denyVision) AllowPixels(context.Context) bool { return false }

type allowVision struct{}

func (allowVision) AllowPixels(context.Context) bool { return true }

func TestBuildMessagesRespectsVisionGate(t *testing.T) {
	tid, _ := valueobject.NewThreadID("thr_vis_gate")
	turn, _ := valueobject.NewTurnID("trn_vis_gate")
	png := tinyPNG()
	store := &memAttachments{byID: map[string]struct {
		meta entity.AttachmentMeta
		data []byte
	}{
		"att_1": {
			meta: entity.AttachmentMeta{ID: "att_1", Kind: "image", Mime: "image/png"},
			data: png,
		},
	}}
	items := []entity.TranscriptItem{{
		Type: enum.ItemUserMessage, ThreadID: tid, TurnID: turn,
		Payload: map[string]any{
			"text": "desc",
			"attachments": []any{
				map[string]any{"attachment_id": "att_1", "kind": "image", "mime": "image/png"},
			},
		},
	}}

	wDeny := New(Deps{Attachments: store, Vision: denyVision{}})
	msgs := wDeny.buildMessages(context.Background(), tid, items)
	if _, ok := msgs[0]["content"].(string); !ok {
		t.Fatalf("deny gate should strip pixels: %#v", msgs[0]["content"])
	}

	wAllow := New(Deps{Attachments: store, Vision: allowVision{}})
	msgs = wAllow.buildMessages(context.Background(), tid, items)
	parts, ok := msgs[0]["content"].([]map[string]any)
	if !ok || len(parts) < 2 || parts[1]["type"] != "image_url" {
		t.Fatalf("allow gate content=%#v", msgs[0]["content"])
	}
	img, _ := parts[1]["image_url"].(map[string]any)
	url, _ := img["url"].(string)
	if !strings.HasPrefix(url, "data:image/") {
		t.Fatalf("must be data URL from store, got %q", url[:min(48, len(url))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
