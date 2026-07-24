package attachments

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/repository"
	"buatpostingan/internal/domain/valueobject"
	"buatpostingan/internal/pkg/apperr"
	"buatpostingan/internal/pkg/idgen"
)

var _ repository.AttachmentStore = (*Store)(nil)

const (
	DefaultMaxBytes int64 = 8 << 20 // 8 MiB
	metaSuffix            = ".meta.json"
	dataSuffix            = ".data"
)

// Store keeps uploads under {root}/attachments/{threadID}/.
type Store struct {
	root     string
	maxBytes int64
}

func NewStore(storageRoot string, maxBytes int64) (*Store, error) {
	abs, err := filepath.Abs(storageRoot)
	if err != nil {
		return nil, fmt.Errorf("attachments: storage root: %w", err)
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBytes
	}
	root := filepath.Join(abs, "attachments")
	if err := os.MkdirAll(root, 0o775); err != nil {
		return nil, err
	}
	return &Store{root: root, maxBytes: maxBytes}, nil
}

func (s *Store) Save(_ context.Context, in repository.SaveAttachmentInput) (entity.AttachmentMeta, error) {
	if in.ThreadID.String() == "" {
		return entity.AttachmentMeta{}, apperr.Validation("thread_id required")
	}
	if int64(len(in.Data)) == 0 {
		return entity.AttachmentMeta{}, apperr.Validation("empty file")
	}
	if int64(len(in.Data)) > s.maxBytes {
		return entity.AttachmentMeta{}, apperr.New(413, apperr.CodeValidation, fmt.Sprintf("file too large (max %d bytes)", s.maxBytes))
	}

	filename := sanitizeFilename(in.Filename)
	mime := strings.TrimSpace(strings.ToLower(in.Mime))
	if mime == "" {
		mime = sniffMime(filename, in.Data)
	}
	kind, ok := classify(filename, mime)
	if !ok {
		return entity.AttachmentMeta{}, apperr.Validation("file type not allowed")
	}

	id := idgen.New("att")
	dir, err := s.threadDir(in.ThreadID)
	if err != nil {
		return entity.AttachmentMeta{}, err
	}
	if err := os.MkdirAll(dir, 0o775); err != nil {
		return entity.AttachmentMeta{}, apperr.New(500, apperr.CodeInternal, "could not create attachment dir")
	}

	meta := entity.AttachmentMeta{
		ID:           id,
		ThreadID:     in.ThreadID,
		Filename:     filename,
		Mime:         mime,
		Size:         int64(len(in.Data)),
		Kind:         kind,
		StoredName:   id + dataSuffix,
		UploadedAt:   time.Now().UTC(),
		UploadedByID: in.UploadedByID,
	}
	if kind == "image" {
		cfg, _, cfgErr := image.DecodeConfig(bytes.NewReader(in.Data))
		if cfgErr == nil {
			meta.Width = cfg.Width
			meta.Height = cfg.Height
		}
	}

	dataPath := filepath.Join(dir, meta.StoredName)
	metaPath := filepath.Join(dir, id+metaSuffix)
	if err := os.WriteFile(dataPath, in.Data, 0o664); err != nil {
		return entity.AttachmentMeta{}, apperr.New(500, apperr.CodeInternal, "could not store attachment")
	}
	raw, err := json.Marshal(metaFile(meta))
	if err != nil {
		_ = os.Remove(dataPath)
		return entity.AttachmentMeta{}, err
	}
	if err := os.WriteFile(metaPath, raw, 0o664); err != nil {
		_ = os.Remove(dataPath)
		return entity.AttachmentMeta{}, apperr.New(500, apperr.CodeInternal, "could not store attachment meta")
	}
	return meta, nil
}

func (s *Store) Get(ctx context.Context, threadID valueobject.ThreadID, attachmentID string) (entity.AttachmentMeta, []byte, error) {
	meta, path, err := s.ResolvePath(ctx, threadID, attachmentID)
	if err != nil {
		return entity.AttachmentMeta{}, nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return entity.AttachmentMeta{}, nil, apperr.NotFound("attachment not found")
	}
	return meta, data, nil
}

func (s *Store) List(_ context.Context, threadID valueobject.ThreadID) ([]entity.AttachmentMeta, error) {
	dir, err := s.threadDir(threadID)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []entity.AttachmentMeta{}, nil
		}
		return nil, err
	}
	out := make([]entity.AttachmentMeta, 0)
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, metaSuffix) {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		var mf metaWire
		if err := json.Unmarshal(raw, &mf); err != nil {
			continue
		}
		out = append(out, mf.toEntity(threadID))
	}
	return out, nil
}

func (s *Store) ResolvePath(_ context.Context, threadID valueobject.ThreadID, attachmentID string) (entity.AttachmentMeta, string, error) {
	id, err := sanitizeAttachmentID(attachmentID)
	if err != nil {
		return entity.AttachmentMeta{}, "", apperr.Validation("invalid attachment_id")
	}
	dir, err := s.threadDir(threadID)
	if err != nil {
		return entity.AttachmentMeta{}, "", err
	}
	metaPath := filepath.Join(dir, id+metaSuffix)
	raw, err := os.ReadFile(metaPath)
	if err != nil {
		return entity.AttachmentMeta{}, "", apperr.NotFound("attachment not found")
	}
	var mf metaWire
	if err := json.Unmarshal(raw, &mf); err != nil {
		return entity.AttachmentMeta{}, "", apperr.NotFound("attachment not found")
	}
	meta := mf.toEntity(threadID)
	dataPath := filepath.Join(dir, meta.StoredName)
	if !underRoot(dir, dataPath) {
		return entity.AttachmentMeta{}, "", apperr.Validation("invalid attachment path")
	}
	if _, err := os.Stat(dataPath); err != nil {
		return entity.AttachmentMeta{}, "", apperr.NotFound("attachment not found")
	}
	return meta, dataPath, nil
}

func (s *Store) threadDir(threadID valueobject.ThreadID) (string, error) {
	tid := threadID.String()
	if tid == "" || strings.Contains(tid, "..") || strings.ContainsAny(tid, `/\`) {
		return "", apperr.Validation("invalid thread_id")
	}
	dir := filepath.Join(s.root, tid)
	if !underRoot(s.root, dir) {
		return "", apperr.Validation("invalid thread_id")
	}
	return dir, nil
}

type metaWire struct {
	ID           string    `json:"id"`
	Filename     string    `json:"filename"`
	Mime         string    `json:"mime"`
	Size         int64     `json:"size"`
	Kind         string    `json:"kind"`
	StoredName   string    `json:"stored_name"`
	Width        int       `json:"width,omitempty"`
	Height       int       `json:"height,omitempty"`
	UploadedAt   time.Time `json:"uploaded_at"`
	UploadedByID int64     `json:"uploaded_by_admin_user_id"`
}

func metaFile(m entity.AttachmentMeta) metaWire {
	return metaWire{
		ID:           m.ID,
		Filename:     m.Filename,
		Mime:         m.Mime,
		Size:         m.Size,
		Kind:         m.Kind,
		StoredName:   m.StoredName,
		Width:        m.Width,
		Height:       m.Height,
		UploadedAt:   m.UploadedAt,
		UploadedByID: m.UploadedByID,
	}
}

func (m metaWire) toEntity(threadID valueobject.ThreadID) entity.AttachmentMeta {
	return entity.AttachmentMeta{
		ID:           m.ID,
		ThreadID:     threadID,
		Filename:     m.Filename,
		Mime:         m.Mime,
		Size:         m.Size,
		Kind:         m.Kind,
		StoredName:   m.StoredName,
		Width:        m.Width,
		Height:       m.Height,
		UploadedAt:   m.UploadedAt,
		UploadedByID: m.UploadedByID,
	}
}

func underRoot(root, candidate string) bool {
	absRoot, err1 := filepath.Abs(root)
	absCand, err2 := filepath.Abs(candidate)
	if err1 != nil || err2 != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absCand)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func sanitizeAttachmentID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" || !strings.HasPrefix(id, "att_") {
		return "", fmt.Errorf("bad id")
	}
	for _, r := range id {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			continue
		}
		return "", fmt.Errorf("bad id")
	}
	return id, nil
}

func sanitizeFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, "\x00", "")
	if name == "" || name == "." || name == ".." {
		return "upload.bin"
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(r)
		case r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" {
		return "upload.bin"
	}
	if len(out) > 120 {
		out = out[:120]
	}
	return out
}

var textExt = map[string]bool{
	".md": true, ".markdown": true, ".txt": true, ".html": true, ".htm": true,
	".json": true, ".csv": true, ".xml": true, ".yaml": true, ".yml": true,
	".toml": true, ".log": true, ".css": true, ".js": true, ".ts": true,
	".go": true, ".py": true, ".rs": true, ".sh": true, ".env": true,
	".ini": true, ".cfg": true, ".conf": true, ".svg": true,
}

var imageExt = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
}

func classify(filename, mime string) (kind string, ok bool) {
	ext := strings.ToLower(filepath.Ext(filename))
	mime = strings.ToLower(strings.TrimSpace(mime))
	switch {
	case imageExt[ext] || strings.HasPrefix(mime, "image/"):
		if imageExt[ext] || mime == "image/png" || mime == "image/jpeg" || mime == "image/gif" || mime == "image/webp" {
			return "image", true
		}
	case textExt[ext] || strings.HasPrefix(mime, "text/") ||
		mime == "application/json" || mime == "application/xml" ||
		mime == "application/yaml" || mime == "application/x-yaml" ||
		mime == "application/javascript":
		return "text", true
	}
	return "", false
}

func sniffMime(filename string, data []byte) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".json":
		return "application/json"
	case ".html", ".htm":
		return "text/html"
	case ".csv":
		return "text/csv"
	case ".xml":
		return "application/xml"
	case ".yaml", ".yml":
		return "application/yaml"
	case ".md", ".markdown", ".txt":
		return "text/plain"
	}
	if len(data) >= 8 {
		switch {
		case bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4e, 0x47}):
			return "image/png"
		case bytes.HasPrefix(data, []byte{0xff, 0xd8, 0xff}):
			return "image/jpeg"
		case bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a")):
			return "image/gif"
		case bytes.HasPrefix(data, []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")):
			return "image/webp"
		}
	}
	return "application/octet-stream"
}
