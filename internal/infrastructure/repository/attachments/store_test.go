package attachments

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"buatpostingan/internal/domain/repository"
	"buatpostingan/internal/domain/valueobject"
	"buatpostingan/internal/pkg/apperr"
)

func TestSaveListGetAndSandbox(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	store, err := NewStore(root, 1024)
	if err != nil {
		t.Fatal(err)
	}
	tid, _ := valueobject.NewThreadID("thr_testattach1")

	png := tinyPNG()
	meta, err := store.Save(context.Background(), repository.SaveAttachmentInput{
		ThreadID:     tid,
		Filename:     "shot.png",
		Mime:         "image/png",
		Data:         png,
		UploadedByID: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if meta.ID == "" || meta.Kind != "image" {
		t.Fatalf("meta=%+v", meta)
	}
	if meta.Width == 0 || meta.Height == 0 {
		t.Fatalf("expected dimensions, got %dx%d", meta.Width, meta.Height)
	}

	list, err := store.List(context.Background(), tid)
	if err != nil || len(list) != 1 {
		t.Fatalf("list=%v err=%v", list, err)
	}

	got, data, err := store.Get(context.Background(), tid, meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Filename != "shot.png" || !bytes.Equal(data, png) {
		t.Fatalf("get mismatch %+v", got)
	}

	_, _, err = store.Get(context.Background(), tid, "../etc/passwd")
	if err == nil {
		t.Fatal("expected path escape reject")
	}
	if ae, ok := err.(*apperr.Error); !ok || ae.Code != apperr.CodeValidation {
		t.Fatalf("want validation, got %v", err)
	}

	_, err = store.Save(context.Background(), repository.SaveAttachmentInput{
		ThreadID: tid,
		Filename: "x.exe",
		Mime:     "application/octet-stream",
		Data:     []byte("MZ"),
	})
	if err == nil {
		t.Fatal("expected type reject")
	}

	big := bytes.Repeat([]byte("a"), 2048)
	_, err = store.Save(context.Background(), repository.SaveAttachmentInput{
		ThreadID: tid,
		Filename: "big.txt",
		Mime:     "text/plain",
		Data:     big,
	})
	if err == nil {
		t.Fatal("expected size reject")
	}

	// Ensure files land under attachments/{thread}/
	dir := filepath.Join(root, "attachments", tid.String())
	entries, _ := os.ReadDir(dir)
	if len(entries) < 2 {
		t.Fatalf("expected meta+data files, got %d", len(entries))
	}
}

func TestTextAttachment(t *testing.T) {
	t.Parallel()
	store, err := NewStore(t.TempDir(), 0)
	if err != nil {
		t.Fatal(err)
	}
	tid, _ := valueobject.NewThreadID("thr_text1")
	meta, err := store.Save(context.Background(), repository.SaveAttachmentInput{
		ThreadID: tid,
		Filename: "notes.md",
		Mime:     "text/markdown",
		Data:     []byte("# Hello\nworld"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if meta.Kind != "text" {
		t.Fatalf("kind=%s", meta.Kind)
	}
}

// 1x1 PNG
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
