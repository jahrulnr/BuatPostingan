package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"buatpostingan/internal/domain/entity"
)

func TestModelImporter_OpenAIFormat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer sk-openai" {
			t.Errorf("Authorization=%q", auth)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data": []map[string]any{
				{"id": "gpt-4o", "object": "model", "created": 1710000000, "owned_by": "openai"},
				{"id": "gpt-4o-mini", "object": "model", "created": 1710000001, "owned_by": "openai"},
			},
		})
	}))
	defer srv.Close()

	imp := NewModelImporter()
	got, err := imp.ImportModels(context.Background(), entity.SettingsProvider{
		BaseURL: srv.URL + "/v1",
		APIKey:  "sk-openai",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 models, got %d", len(got))
	}
	if got[0].ID != "gpt-4o" || got[0].Label != "gpt-4o" {
		t.Fatalf("first model: %+v", got[0])
	}
}

func TestModelImporter_AnthropicFormat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("x-api-key") != "sk-anthropic" {
			t.Errorf("x-api-key=%q", r.Header.Get("x-api-key"))
		}
		after := r.URL.Query().Get("after_id")
		if after == "" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "claude-1", "type": "model", "display_name": "Claude 1", "created_at": "2024-01-01T00:00:00Z"},
					{"id": "claude-2", "type": "model", "display_name": "Claude 2", "created_at": "2024-02-01T00:00:00Z"},
				},
				"has_more": true,
				"first_id": "claude-1",
				"last_id":  "claude-2",
			})
			return
		}
		if after != "claude-2" {
			t.Errorf("unexpected after_id=%q", after)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "claude-3", "type": "model", "display_name": "Claude 3", "created_at": "2024-03-01T00:00:00Z"},
			},
			"has_more": false,
			"first_id": "claude-3",
			"last_id":  "claude-3",
		})
	}))
	defer srv.Close()

	imp := NewModelImporter()
	imp.PageSize = 2
	got, err := imp.ImportModels(context.Background(), entity.SettingsProvider{
		BaseURL: srv.URL + "/v1",
		APIKey:  "sk-anthropic",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 models, got %d: %+v", len(got), got)
	}
	if got[2].ID != "claude-3" || got[2].Label != "Claude 3" {
		t.Fatalf("last model: %+v", got[2])
	}
}

func TestModelImporter_OpenRouterFormat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/models" {
			http.NotFound(w, r)
			return
		}
		offset := r.URL.Query().Get("offset")
		if offset == "" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "openai/gpt-4o", "name": "GPT-4o", "created": 1710000000},
				},
				"total_count": 2,
				"links":       map[string]any{"next": "/api/v1/models?offset=1&limit=1"},
			})
			return
		}
		if offset != "1" {
			t.Errorf("unexpected offset=%q", offset)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "anthropic/claude-3.5-sonnet", "name": "Claude 3.5 Sonnet", "created": 1710000001},
			},
			"total_count": 2,
			"links":       map[string]any{"next": nil},
		})
	}))
	defer srv.Close()

	imp := NewModelImporter()
	got, err := imp.ImportModels(context.Background(), entity.SettingsProvider{
		BaseURL: srv.URL + "/api/v1",
		APIKey:  "sk-or",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 models, got %d: %+v", len(got), got)
	}
	if got[0].ID != "anthropic/claude-3.5-sonnet" || got[0].Label != "Claude 3.5 Sonnet" {
		t.Fatalf("first model: %+v", got[0])
	}
}

func TestModelImporter_PlainArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[
			{"id":"m1"},
			{"id":"m2","name":"Model 2"}
		]`))
	}))
	defer srv.Close()

	imp := NewModelImporter()
	got, err := imp.ImportModels(context.Background(), entity.SettingsProvider{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1].Label != "Model 2" {
		t.Fatalf("got: %+v", got)
	}
}

func TestModelImporter_SkipsNonModelObjects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "keep", "object": "model"},
				{"id": "drop", "object": "foo"},
				{"id": "keep2", "type": "model"},
				{"id": "drop2", "type": "foo"},
			},
		})
	}))
	defer srv.Close()

	imp := NewModelImporter()
	got, err := imp.ImportModels(context.Background(), entity.SettingsProvider{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 models, got %d: %+v", len(got), got)
	}
}

func TestModelImporter_Deduplicates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "dup"},
				{"id": "dup"},
				{"id": "unique"},
			},
		})
	}))
	defer srv.Close()

	imp := NewModelImporter()
	got, err := imp.ImportModels(context.Background(), entity.SettingsProvider{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 models, got %d: %+v", len(got), got)
	}
}

func TestModelImporter_UpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"bad key"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	imp := NewModelImporter()
	_, err := imp.ImportModels(context.Background(), entity.SettingsProvider{BaseURL: srv.URL})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("error should mention 401: %v", err)
	}
}

func TestModelImporter_MissingBaseURL(t *testing.T) {
	imp := NewModelImporter()
	_, err := imp.ImportModels(context.Background(), entity.SettingsProvider{BaseURL: " "})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestModelImporter_MaxPagesGuard(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":        []map[string]any{{"id": fmt.Sprintf("m%d", calls)}},
			"total_count": 1000,
			"links":       map[string]any{"next": "/models?offset=" + fmt.Sprint(calls)},
		})
	}))
	defer srv.Close()

	imp := NewModelImporter()
	imp.MaxPages = 3
	got, err := imp.ImportModels(context.Background(), entity.SettingsProvider{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Fatalf("want 3 calls, got %d", calls)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 models, got %d", len(got))
	}
}

func contains(slice []string, want string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, want) {
			return true
		}
	}
	return false
}

func TestModelImporter_AnthropicMetadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id":               "claude-opus-4-6",
					"type":             "model",
					"display_name":     "Claude Opus 4.6",
					"max_input_tokens": 200000,
					"max_tokens":       32000,
					"capabilities": map[string]any{
						"image_input": map[string]any{"supported": true},
						"pdf_input":   map[string]any{"supported": true},
						"effort": map[string]any{
							"supported": true,
							"low":       map[string]any{"supported": true},
							"medium":    map[string]any{"supported": true},
							"high":      map[string]any{"supported": true},
							"max":       map[string]any{"supported": true},
							"xhigh":     map[string]any{"supported": true},
						},
						"structured_outputs": map[string]any{"supported": true},
					},
				},
			},
			"has_more": false,
		})
	}))
	defer srv.Close()

	imp := NewModelImporter()
	got, err := imp.ImportModels(context.Background(), entity.SettingsProvider{
		BaseURL: srv.URL + "/v1",
		APIKey:  "sk-anthropic",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 model, got %d", len(got))
	}
	m := got[0]
	if m.Label != "Claude Opus 4.6" {
		t.Fatalf("label: %q", m.Label)
	}
	if m.ContextWindow != 200000 {
		t.Fatalf("context window: %d", m.ContextWindow)
	}
	if m.MaxOutput != 32000 {
		t.Fatalf("max output: %d", m.MaxOutput)
	}
	if !contains(m.InputModes, "image") || !contains(m.InputModes, "pdf") {
		t.Fatalf("input modes: %+v", m.InputModes)
	}
	if !contains(m.EffortLevels, "low") || !contains(m.EffortLevels, "high") || !contains(m.EffortLevels, "xhigh") {
		t.Fatalf("effort levels: %+v", m.EffortLevels)
	}
}

func TestModelImporter_OpenRouterMetadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/models" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id":             "openai/gpt-4o",
					"name":           "GPT-4o",
					"context_length": 128000,
					"description":    "Multimodal model",
					"architecture": map[string]any{
						"modality":          "text+image->text",
						"input_modalities":  []string{"text", "image"},
						"output_modalities": []string{"text"},
					},
					"supported_parameters": []string{"temperature", "tools", "tool_choice", "reasoning"},
					"reasoning": map[string]any{
						"supported_efforts": []string{"high", "medium", "low", "minimal"},
						"default_effort":    "medium",
					},
				},
			},
			"links": map[string]any{"next": nil},
		})
	}))
	defer srv.Close()

	imp := NewModelImporter()
	got, err := imp.ImportModels(context.Background(), entity.SettingsProvider{
		BaseURL: srv.URL + "/api/v1",
		APIKey:  "sk-or",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 model, got %d", len(got))
	}
	m := got[0]
	if m.ContextWindow != 128000 {
		t.Fatalf("context window: %d", m.ContextWindow)
	}
	if !contains(m.InputModes, "text") || !contains(m.InputModes, "image") {
		t.Fatalf("input modes: %+v", m.InputModes)
	}
	if !contains(m.OutputModes, "text") {
		t.Fatalf("output modes: %+v", m.OutputModes)
	}
	if !m.SupportsTools {
		t.Fatal("should support tools")
	}
	if !contains(m.EffortLevels, "high") || !contains(m.EffortLevels, "minimal") {
		t.Fatalf("effort levels: %+v", m.EffortLevels)
	}
	if m.Description != "Multimodal model" {
		t.Fatalf("description: %q", m.Description)
	}
}

func TestModelImporter_ModalityShorthand(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id": "m1",
					"architecture": map[string]any{
						"modality": "text+image+file->text+image",
					},
				},
			},
		})
	}))
	defer srv.Close()

	imp := NewModelImporter()
	got, err := imp.ImportModels(context.Background(), entity.SettingsProvider{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 model, got %d", len(got))
	}
	m := got[0]
	if !contains(m.InputModes, "text") || !contains(m.InputModes, "image") || !contains(m.InputModes, "file") {
		t.Fatalf("input modes from shorthand: %+v", m.InputModes)
	}
	if !contains(m.OutputModes, "text") || !contains(m.OutputModes, "image") {
		t.Fatalf("output modes from shorthand: %+v", m.OutputModes)
	}
}

func TestModelListItemCarriesRecognizedTaskMetadata(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		item modelListItem
		want string
	}{
		{name: "explicit task", item: modelListItem{ID: "opaque", Task: "speech-to-text"}, want: "speech-to-text"},
		{name: "provider type", item: modelListItem{ID: "opaque", Type: "embedding"}, want: "embedding"},
		{name: "generic model type", item: modelListItem{ID: "chat", Type: "model"}, want: ""},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.item.ToEntity().Task; got != tt.want {
				t.Fatalf("task=%q want=%q", got, tt.want)
			}
		})
	}
}
