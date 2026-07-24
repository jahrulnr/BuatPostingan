package httpdelivery

import (
	"net/http"
	"strconv"

	"buatpostingan/internal/application/usecase/webchat"
	"buatpostingan/delivery/presenter"
	"buatpostingan/internal/domain/valueobject"
	"buatpostingan/internal/pkg/apperr"
)

// WebchatHandler is the thin HTTP adapter for /api/webchat.
type WebchatHandler struct {
	UC *webchat.Usecase
}

func NewWebchatHandler(uc *webchat.Usecase) *WebchatHandler {
	return &WebchatHandler{UC: uc}
}

func (h *WebchatHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/webchat/conversations", h.ListConversations)
	mux.HandleFunc("POST /api/webchat/threads", h.CreateThread)
	mux.HandleFunc("GET /api/webchat/threads/{threadID}", h.GetThread)
	mux.HandleFunc("PATCH /api/webchat/threads/{threadID}", h.RenameThread)
	mux.HandleFunc("POST /api/webchat/threads/{threadID}/turns", h.StartTurn)
	mux.HandleFunc("POST /api/webchat/threads/{threadID}/retry", h.RetryTurn)
	mux.HandleFunc("POST /api/webchat/threads/{threadID}/interrupt", h.InterruptTurn)
	mux.HandleFunc("GET /api/webchat/threads/{threadID}/events", h.Events)
}

func (h *WebchatHandler) ListConversations(w http.ResponseWriter, r *http.Request) {
	out, err := h.UC.ListConversations(r.Context())
	if err != nil {
		presenter.WriteAppError(w, err)
		return
	}
	presenter.WriteJSON(w, http.StatusOK, map[string]any{
		"conversations": out.Conversations,
		"docs_index":    out.DocsIndex,
	})
}

func (h *WebchatHandler) CreateThread(w http.ResponseWriter, r *http.Request) {
	snap, err := h.UC.CreateThread(r.Context(), adminUserID(r))
	if err != nil {
		presenter.WriteAppError(w, err)
		return
	}
	presenter.WriteJSON(w, http.StatusCreated, map[string]any{
		"thread_id":                 snap.ThreadID.String(),
		"seq_head":                  snap.SeqHead,
		"created_by_admin_user_id":  adminUserID(r),
	})
}

func (h *WebchatHandler) GetThread(w http.ResponseWriter, r *http.Request) {
	tid, err := valueobject.NewThreadID(r.PathValue("threadID"))
	if err != nil {
		presenter.WriteAppError(w, err)
		return
	}
	after, _ := strconv.ParseUint(r.URL.Query().Get("after_seq"), 10, 64)
	snap, err := h.UC.GetThread(r.Context(), tid, after)
	if err != nil {
		presenter.WriteAppError(w, err)
		return
	}
	presenter.WriteJSON(w, http.StatusOK, snap)
}

func (h *WebchatHandler) RenameThread(w http.ResponseWriter, r *http.Request) {
	tid, err := valueobject.NewThreadID(r.PathValue("threadID"))
	if err != nil {
		presenter.WriteAppError(w, err)
		return
	}
	var body struct {
		Title string `json:"title"`
	}
	if err := decodeJSON(r, &body); err != nil {
		presenter.WriteAppError(w, err)
		return
	}
	title, err := valueobject.NewTitle(body.Title)
	if err != nil {
		presenter.WriteAppError(w, err)
		return
	}
	if err := h.UC.RenameThread(r.Context(), tid, title); err != nil {
		presenter.WriteAppError(w, err)
		return
	}
	presenter.WriteJSON(w, http.StatusOK, map[string]any{
		"thread_id": tid.String(),
		"title":     title.String(),
	})
}

func (h *WebchatHandler) StartTurn(w http.ResponseWriter, r *http.Request) {
	tid, err := valueobject.NewThreadID(r.PathValue("threadID"))
	if err != nil {
		presenter.WriteAppError(w, err)
		return
	}
	var body struct {
		Message string `json:"message"`
	}
	if err := decodeJSON(r, &body); err != nil {
		presenter.WriteAppError(w, err)
		return
	}
	out, err := h.UC.StartTurn(r.Context(), webchat.StartTurnInput{
		ThreadID:    tid,
		Message:     body.Message,
		AdminUserID: adminUserID(r),
		AdminName:   adminDisplayName(r),
	})
	if err != nil {
		presenter.WriteAppError(w, err)
		return
	}
	presenter.WriteJSON(w, http.StatusAccepted, map[string]any{
		"thread_id":              out.ThreadID.String(),
		"turn_id":                out.TurnID.String(),
		"seq_head":               out.SeqHead,
		"status":                 out.Status,
		"floor_holder_admin_id":  out.FloorHolderAdminID,
		"floor_remaining_sec":    out.FloorRemainingSec,
	})
}

func (h *WebchatHandler) RetryTurn(w http.ResponseWriter, r *http.Request) {
	tid, err := valueobject.NewThreadID(r.PathValue("threadID"))
	if err != nil {
		presenter.WriteAppError(w, err)
		return
	}
	var body struct {
		TurnID string `json:"turn_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		presenter.WriteAppError(w, err)
		return
	}
	turnID, err := valueobject.NewTurnID(body.TurnID)
	if err != nil {
		presenter.WriteAppError(w, err)
		return
	}
	out, err := h.UC.RetryTurn(r.Context(), tid, turnID, adminUserID(r))
	if err != nil {
		presenter.WriteAppError(w, err)
		return
	}
	presenter.WriteJSON(w, http.StatusAccepted, map[string]any{
		"thread_id": out.ThreadID.String(),
		"turn_id":   out.TurnID.String(),
		"seq_head":  out.SeqHead,
		"status":    out.Status,
	})
}

func (h *WebchatHandler) InterruptTurn(w http.ResponseWriter, r *http.Request) {
	tid, err := valueobject.NewThreadID(r.PathValue("threadID"))
	if err != nil {
		presenter.WriteAppError(w, err)
		return
	}
	var body struct {
		TurnID string `json:"turn_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		presenter.WriteAppError(w, err)
		return
	}
	turnID, err := valueobject.NewTurnID(body.TurnID)
	if err != nil {
		presenter.WriteAppError(w, err)
		return
	}
	if err := h.UC.InterruptTurn(r.Context(), tid, turnID, adminUserID(r)); err != nil {
		presenter.WriteAppError(w, err)
		return
	}
	presenter.WriteJSON(w, http.StatusAccepted, map[string]any{
		"ok":     true,
		"status": "interrupt_requested",
	})
}

func (h *WebchatHandler) Events(w http.ResponseWriter, r *http.Request) {
	_ = r.PathValue("threadID")
	presenter.WriteAppError(w, apperr.NotImplemented("SSE EventStreamer"))
}
