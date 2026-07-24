package httpdelivery

import (
	"net/http"

	"buatpostingan/delivery/presenter"
	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/usecase/settings"
)

// SettingsHandler is the thin HTTP adapter for /api/settings.
type SettingsHandler struct {
	UC *settings.Service
}

func NewSettingsHandler(uc *settings.Service) *SettingsHandler {
	return &SettingsHandler{UC: uc}
}

func (h *SettingsHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/settings", h.GetSnapshot)
	mux.HandleFunc("GET /api/settings/users", h.ListUsers)
	mux.HandleFunc("POST /api/settings/users", h.CreateUser)
	mux.HandleFunc("PATCH /api/settings/users/{id}", h.UpdateUser)
	mux.HandleFunc("DELETE /api/settings/users/{id}", h.DeleteUser)
	mux.HandleFunc("GET /api/settings/llm/providers", h.ListProviders)
	mux.HandleFunc("POST /api/settings/llm/providers", h.CreateProvider)
	mux.HandleFunc("GET /api/settings/llm/providers/{id}", h.GetProvider)
	mux.HandleFunc("PATCH /api/settings/llm/providers/{id}", h.UpdateProvider)
	mux.HandleFunc("DELETE /api/settings/llm/providers/{id}", h.DeleteProvider)
	mux.HandleFunc("POST /api/settings/llm/providers/{id}/models", h.AddModel)
	mux.HandleFunc("DELETE /api/settings/llm/providers/{id}/models/{modelId}", h.RemoveModel)
	mux.HandleFunc("POST /api/settings/llm/providers/{id}/import-models", h.ImportModels)
}

func (h *SettingsHandler) GetSnapshot(w http.ResponseWriter, r *http.Request) {
	out, err := h.UC.GetSnapshot(r.Context())
	if err != nil {
		writeErr(w, r, "settings", err)
		return
	}
	presenter.WriteJSON(w, http.StatusOK, out)
}

func (h *SettingsHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	out, err := h.UC.ListUsers(r.Context())
	if err != nil {
		writeErr(w, r, "settings", err)
		return
	}
	presenter.WriteJSON(w, http.StatusOK, map[string]any{"users": out})
}

func (h *SettingsHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var body settings.UserInput
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, "settings", err)
		return
	}
	out, err := h.UC.CreateUser(r.Context(), body)
	if err != nil {
		writeErr(w, r, "settings", err)
		return
	}
	presenter.WriteJSON(w, http.StatusCreated, out)
}

func (h *SettingsHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	var body settings.UserInput
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, "settings", err)
		return
	}
	out, err := h.UC.UpdateUser(r.Context(), r.PathValue("id"), body)
	if err != nil {
		writeErr(w, r, "settings", err)
		return
	}
	presenter.WriteJSON(w, http.StatusOK, out)
}

func (h *SettingsHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	if err := h.UC.DeleteUser(r.Context(), r.PathValue("id")); err != nil {
		writeErr(w, r, "settings", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SettingsHandler) ListProviders(w http.ResponseWriter, r *http.Request) {
	out, err := h.UC.ListProviders(r.Context())
	if err != nil {
		writeErr(w, r, "settings", err)
		return
	}
	presenter.WriteJSON(w, http.StatusOK, map[string]any{"providers": out})
}

func (h *SettingsHandler) GetProvider(w http.ResponseWriter, r *http.Request) {
	out, err := h.UC.GetProvider(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, r, "settings", err)
		return
	}
	presenter.WriteJSON(w, http.StatusOK, out)
}

func (h *SettingsHandler) CreateProvider(w http.ResponseWriter, r *http.Request) {
	var body settings.ProviderInput
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, "settings", err)
		return
	}
	out, err := h.UC.CreateProvider(r.Context(), body)
	if err != nil {
		writeErr(w, r, "settings", err)
		return
	}
	presenter.WriteJSON(w, http.StatusCreated, out)
}

func (h *SettingsHandler) UpdateProvider(w http.ResponseWriter, r *http.Request) {
	var body settings.ProviderInput
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, "settings", err)
		return
	}
	out, err := h.UC.UpdateProvider(r.Context(), r.PathValue("id"), body)
	if err != nil {
		writeErr(w, r, "settings", err)
		return
	}
	presenter.WriteJSON(w, http.StatusOK, out)
}

func (h *SettingsHandler) DeleteProvider(w http.ResponseWriter, r *http.Request) {
	if err := h.UC.DeleteProvider(r.Context(), r.PathValue("id")); err != nil {
		writeErr(w, r, "settings", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SettingsHandler) AddModel(w http.ResponseWriter, r *http.Request) {
	var body entity.SettingsModel
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, "settings", err)
		return
	}
	out, err := h.UC.AddModel(r.Context(), r.PathValue("id"), body)
	if err != nil {
		writeErr(w, r, "settings", err)
		return
	}
	presenter.WriteJSON(w, http.StatusOK, out)
}

func (h *SettingsHandler) RemoveModel(w http.ResponseWriter, r *http.Request) {
	out, err := h.UC.RemoveModel(r.Context(), r.PathValue("id"), r.PathValue("modelId"))
	if err != nil {
		writeErr(w, r, "settings", err)
		return
	}
	presenter.WriteJSON(w, http.StatusOK, out)
}

func (h *SettingsHandler) ImportModels(w http.ResponseWriter, r *http.Request) {
	out, err := h.UC.ImportModelsStub(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, r, "settings", err)
		return
	}
	presenter.WriteJSON(w, http.StatusOK, out)
}
