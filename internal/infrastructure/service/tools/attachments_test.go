package tools_test

import (
	"context"
	"path/filepath"
	"testing"

	"buatpostingan/internal/domain/repository"
	"buatpostingan/internal/domain/service"
	"buatpostingan/internal/domain/valueobject"
	"buatpostingan/internal/infrastructure/repository/attachments"
	"buatpostingan/internal/infrastructure/service/docs"
	"buatpostingan/internal/infrastructure/service/tools"
)

func TestReadAttachmentAndImage(t *testing.T) {
	repoRoot := findRepoRoot(t)
	docsRoot := filepath.Join(repoRoot, "docs", "webchat")
	toolsRoot := filepath.Join(repoRoot, "resources", "webchat", "tools")
	storage := t.TempDir()

	idx, err := docs.NewIndex(docsRoot, storage, docs.Options{})
	if err != nil {
		t.Fatal(err)
	}
	att, err := attachments.NewStore(storage, 0)
	if err != nil {
		t.Fatal(err)
	}
	tid, _ := valueobject.NewThreadID("thr_toolatt1")
	textMeta, err := att.Save(context.Background(), repository.SaveAttachmentInput{
		ThreadID: tid,
		Filename: "brief.md",
		Mime:     "text/markdown",
		Data:     []byte("# Title\nhello attachment"),
	})
	if err != nil {
		t.Fatal(err)
	}
	imgMeta, err := att.Save(context.Background(), repository.SaveAttachmentInput{
		ThreadID: tid,
		Filename: "dot.png",
		Mime:     "image/png",
		Data:     tinyPNGBytes(),
	})
	if err != nil {
		t.Fatal(err)
	}

	reg, err := tools.NewRegistry(toolsRoot, idx, tools.Options{Attachments: att})
	if err != nil {
		t.Fatal(err)
	}
	ctx := tools.WithThreadID(context.Background(), tid)

	env, err := reg.Execute(ctx, service.ToolCall{
		Name:      "read_attachment",
		Arguments: map[string]any{"attachment_id": textMeta.ID},
	})
	if err != nil || !env.OK {
		t.Fatalf("read_attachment: %+v err=%v", env, err)
	}
	data, _ := env.Data.(map[string]any)
	if data["content"] == nil || data["filename"] != "brief.md" {
		t.Fatalf("data=%#v", data)
	}

	env, err = reg.Execute(ctx, service.ToolCall{
		Name:      "read_image",
		Arguments: map[string]any{"attachment_id": imgMeta.ID},
	})
	if err != nil || !env.OK {
		t.Fatalf("read_image: %+v err=%v", env, err)
	}
	data, _ = env.Data.(map[string]any)
	if data["vision_available"] != true || data["mime"] != "image/png" {
		t.Fatalf("image data=%#v", data)
	}
	if data["content_provided_to_model"] != true {
		t.Fatalf("expected content_provided_to_model: %#v", data)
	}

	regOff, err := tools.NewRegistry(toolsRoot, idx, tools.Options{
		Attachments: att,
		Vision:      denyPixels{},
	})
	if err != nil {
		t.Fatal(err)
	}
	env, err = regOff.Execute(ctx, service.ToolCall{
		Name:      "read_image",
		Arguments: map[string]any{"attachment_id": imgMeta.ID},
	})
	if err != nil || !env.OK {
		t.Fatalf("read_image gated: %+v err=%v", env, err)
	}
	data, _ = env.Data.(map[string]any)
	if data["vision_available"] != false || data["content_provided_to_model"] != false {
		t.Fatalf("gate off should clear vision flags: %#v", data)
	}

	// Wrong thread / missing context
	env, _ = reg.Execute(context.Background(), service.ToolCall{
		Name:      "read_attachment",
		Arguments: map[string]any{"attachment_id": textMeta.ID},
	})
	if env.OK {
		t.Fatal("expected missing_thread")
	}

	other, _ := valueobject.NewThreadID("thr_other")
	env, _ = reg.Execute(tools.WithThreadID(context.Background(), other), service.ToolCall{
		Name:      "read_attachment",
		Arguments: map[string]any{"attachment_id": textMeta.ID},
	})
	if env.OK {
		t.Fatal("expected not_found cross-thread")
	}

	// Path escape via attachment_id
	env, _ = reg.Execute(ctx, service.ToolCall{
		Name:      "read_attachment",
		Arguments: map[string]any{"attachment_id": "../etc/passwd"},
	})
	if env.OK {
		t.Fatal("expected reject")
	}
}

func tinyPNGBytes() []byte {
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xde, 0x00, 0x00, 0x00,
		0x0c, 0x49, 0x44, 0x41, 0x54, 0x08, 0xd7, 0x63, 0xf8, 0xff, 0xff, 0x3f,
		0x00, 0x05, 0xfe, 0x02, 0xfe, 0xdc, 0xcc, 0x59, 0xe7, 0x00, 0x00, 0x00,
		0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}
}

type denyPixels struct{}

func (denyPixels) AllowPixels(context.Context) bool { return false }

