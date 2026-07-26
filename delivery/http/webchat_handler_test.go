package httpdelivery_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	httpdelivery "buatpostingan/delivery/http"
	"buatpostingan/internal/config"
	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/enum"
	"buatpostingan/internal/domain/service"
	"buatpostingan/internal/domain/valueobject"
	"buatpostingan/internal/pkg/apperr"
	"buatpostingan/internal/usecase/webchat"
)

type listFake struct {
	out        webchat.ListConversationsResult
	err        error
	browseOut  webchat.BrowseDirResult
	browsePath string
}

func (f *listFake) ListConversations(context.Context) (webchat.ListConversationsResult, error) {
	return f.out, f.err
}
func (f *listFake) CreateThread(context.Context, int64) (webchat.CreateThreadResult, error) {
	return webchat.CreateThreadResult{}, apperr.NotImplemented("x")
}
func (f *listFake) GetThread(context.Context, valueobject.ThreadID, uint64) (entity.ThreadSnapshot, error) {
	return entity.ThreadSnapshot{}, apperr.NotImplemented("x")
}
func (f *listFake) RenameThread(context.Context, valueobject.ThreadID, valueobject.Title) (webchat.RenameResult, error) {
	return webchat.RenameResult{}, apperr.NotImplemented("x")
}
func (f *listFake) DeleteThread(context.Context, valueobject.ThreadID) error {
	return apperr.NotImplemented("x")
}
func (f *listFake) StartTurn(context.Context, webchat.StartTurnInput) (webchat.StartTurnResult, error) {
	return webchat.StartTurnResult{}, apperr.NotImplemented("x")
}
func (f *listFake) RetryTurn(context.Context, webchat.RetryTurnInput) (webchat.StartTurnResult, error) {
	return webchat.StartTurnResult{}, apperr.NotImplemented("x")
}
func (f *listFake) InterruptTurn(context.Context, valueobject.ThreadID, valueobject.TurnID, int64) error {
	return apperr.NotImplemented("x")
}
func (f *listFake) UploadAttachment(context.Context, webchat.UploadAttachmentInput) (entity.AttachmentMeta, error) {
	return entity.AttachmentMeta{}, apperr.NotImplemented("x")
}
func (f *listFake) ListAttachments(context.Context, valueobject.ThreadID) ([]entity.AttachmentMeta, error) {
	return nil, apperr.NotImplemented("x")
}
func (f *listFake) ListModels(context.Context) (entity.ModelsCatalog, error) {
	return entity.ModelsCatalog{}, apperr.NotImplemented("x")
}
func (f *listFake) BrowseDir(_ context.Context, path string) (webchat.BrowseDirResult, error) {
	f.browsePath = path
	return f.browseOut, f.err
}
func (f *listFake) ListPages(context.Context) ([]service.PageWorkspace, error) {
	return nil, apperr.NotImplemented("x")
}
func (f *listFake) PublishPage(context.Context, string) (service.PageWorkspace, error) {
	return service.PageWorkspace{}, apperr.NotImplemented("x")
}
func (f *listFake) UnpublishPage(context.Context, string) (service.PageWorkspace, error) {
	return service.PageWorkspace{}, apperr.NotImplemented("x")
}
func (f *listFake) DeletePage(context.Context, string) error {
	return apperr.NotImplemented("x")
}
func (f *listFake) SubscribeEvents(context.Context, valueobject.ThreadID, uint64, webchat.EventEmitter) error {
	return apperr.NotImplemented("x")
}

func TestListConversationsJSONShape(t *testing.T) {
	t.Parallel()
	tid, _ := valueobject.NewThreadID("thr_test")
	title, _ := valueobject.NewTitle("Hello")
	uc := &listFake{
		out: webchat.ListConversationsResult{
			Conversations: []webchat.ConversationView{{
				Meta: entity.ConversationMeta{
					ThreadID:             tid,
					Title:                &title,
					TitleSource:          enum.TitleAuto,
					Status:               enum.ConversationActive,
					CreatedByAdminUserID: 1,
				},
				FloorRemainingSec: 0,
			}},
			DocsIndex: entity.DocsIndexGate{
				Usable:        true,
				Status:        "ready",
				Message:       "ok",
				DocumentCount: 3,
			},
		},
	}
	srv := httpdelivery.NewServer(config.Config{WebRoot: "web"}, uc, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/webchat/conversations", nil)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["conversations"]; !ok {
		t.Fatalf("missing conversations: %v", body)
	}
	docs, ok := body["docs_index"].(map[string]any)
	if !ok || docs["usable"] != true {
		t.Fatalf("docs_index: %v", body["docs_index"])
	}
}

func TestListConversationsNotImplemented(t *testing.T) {
	t.Parallel()
	uc := &listFake{err: apperr.NotImplemented("ListConversations")}
	srv := httpdelivery.NewServer(config.Config{WebRoot: "web"}, uc, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/webchat/conversations", nil)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestMountWebchatAPIWithoutStatic(t *testing.T) {
	t.Parallel()
	uc := &listFake{
		out: webchat.ListConversationsResult{
			DocsIndex: entity.DocsIndexGate{Usable: true, Status: "ready", Message: "ok"},
		},
	}
	mux := http.NewServeMux()
	httpdelivery.MountWebchatAPI(mux, uc)
	httpdelivery.MountHealthz(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/webchat/conversations", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("api status %d body %s", rec.Code, rec.Body.String())
	}

	hz := httptest.NewRecorder()
	mux.ServeHTTP(hz, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if hz.Code != http.StatusOK {
		t.Fatalf("healthz %d", hz.Code)
	}

	// No MountStaticWeb → unknown paths 404 (API-only host).
	root := httptest.NewRecorder()
	mux.ServeHTTP(root, httptest.NewRequest(http.MethodGet, "/", nil))
	if root.Code != http.StatusNotFound {
		t.Fatalf("expected 404 without static, got %d", root.Code)
	}
}

type modelsFake struct {
	listFake
	cat entity.ModelsCatalog
}

func (f *modelsFake) ListModels(context.Context) (entity.ModelsCatalog, error) {
	return f.cat, nil
}

func TestListModelsJSONShape(t *testing.T) {
	t.Parallel()
	uc := &modelsFake{
		cat: entity.ModelsCatalog{
			Models: []entity.ModelOption{{
				ID: "openai/gpt-4o-mini", Label: "gpt-4o-mini · OPENROUTER", Provider: "OPENROUTER",
				SupportsVision: true, OutputModes: []string{"text"},
				SupportedEfforts: []string{"low", "medium", "high"}, DefaultEffort: "medium",
			}},
			DefaultModelID: "openai/gpt-4o-mini",
			EffortCurrent:  "auto",
			EffortOptions:  []string{"auto", "none", "medium", "high"},
			Stub:           false,
		},
	}
	srv := httpdelivery.NewServer(config.Config{WebRoot: "web"}, uc, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/webchat/models", nil)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["default_model_id"] != "openai/gpt-4o-mini" {
		t.Fatalf("%v", body)
	}
	models, ok := body["models"].([]any)
	if !ok || len(models) != 1 {
		t.Fatalf("models: %v", body["models"])
	}
	first, ok := models[0].(map[string]any)
	if !ok {
		t.Fatalf("model: %T", models[0])
	}
	outputModes, ok := first["output_modes"].([]any)
	if !ok || len(outputModes) != 1 || outputModes[0] != "text" {
		t.Fatalf("output_modes: %v", first["output_modes"])
	}
	effort, ok := body["effort"].(map[string]any)
	if !ok || effort["current"] != "auto" {
		t.Fatalf("effort: %v", body["effort"])
	}
}
func TestBrowseDirDelegatesEmptyPathAndSerializesResult(t *testing.T) {
	t.Parallel()
	uc := &listFake{
		browseOut: webchat.BrowseDirResult{
			Path:   "/workspace",
			Parent: "/",
			Entries: []webchat.BrowseDirEntry{
				{Name: "project-a", Path: "/workspace/project-a"},
				{Name: "project-b", Path: "/workspace/project-b"},
			},
		},
	}
	srv := httpdelivery.NewServer(config.Config{WebRoot: "web"}, uc, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/webchat/browse", nil)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if uc.browsePath != "" {
		t.Fatalf("BrowseDir path = %q, want empty", uc.browsePath)
	}

	var body struct {
		Path    string `json:"path"`
		Parent  string `json:"parent"`
		Entries []struct {
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Path != "/workspace" || body.Parent != "/" {
		t.Fatalf("path/parent = %q/%q", body.Path, body.Parent)
	}
	if len(body.Entries) != 2 || body.Entries[0].Name != "project-a" || body.Entries[0].Path != "/workspace/project-a" || body.Entries[1].Name != "project-b" || body.Entries[1].Path != "/workspace/project-b" {
		t.Fatalf("entries = %#v", body.Entries)
	}
}
