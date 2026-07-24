package httpdelivery

import (
	"io"
	"net/http"
	"strconv"
	"time"

	"buatpostingan/delivery/presenter"
	"buatpostingan/internal/domain/valueobject"
	"buatpostingan/internal/pkg/apperr"
	"buatpostingan/internal/pkg/logging"
	"buatpostingan/internal/usecase/webchat"
)

// WebchatHandler is the thin HTTP adapter for /api/webchat (FE real driver).
type WebchatHandler struct {
	UC webchat.Usecase
}

func NewWebchatHandler(uc webchat.Usecase) *WebchatHandler {
	return &WebchatHandler{UC: uc}
}

func (h *WebchatHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/webchat/conversations", h.ListConversations)
	mux.HandleFunc("GET /api/webchat/models", h.ListModels)
	mux.HandleFunc("POST /api/webchat/threads", h.CreateThread)
	mux.HandleFunc("GET /api/webchat/threads/{threadID}", h.GetThread)
	mux.HandleFunc("PATCH /api/webchat/threads/{threadID}", h.RenameThread)
	mux.HandleFunc("POST /api/webchat/threads/{threadID}/turns", h.StartTurn)
	mux.HandleFunc("POST /api/webchat/threads/{threadID}/attachments", h.UploadAttachment)
	mux.HandleFunc("GET /api/webchat/threads/{threadID}/attachments", h.ListAttachments)
	mux.HandleFunc("POST /api/webchat/threads/{threadID}/retry", h.RetryTurn)
	mux.HandleFunc("POST /api/webchat/threads/{threadID}/interrupt", h.InterruptTurn)
	mux.HandleFunc("GET /api/webchat/threads/{threadID}/events", h.Events)
}

func (h *WebchatHandler) ListConversations(w http.ResponseWriter, r *http.Request) {
	out, err := h.UC.ListConversations(r.Context())
	if err != nil {
		writeErr(w, r, "webchat", err)
		return
	}
	presenter.WriteJSON(w, http.StatusOK, presenter.ListConversations(out))
}

func (h *WebchatHandler) ListModels(w http.ResponseWriter, r *http.Request) {
	out, err := h.UC.ListModels(r.Context())
	if err != nil {
		writeErr(w, r, "webchat", err)
		return
	}
	presenter.WriteJSON(w, http.StatusOK, presenter.ListModels(out))
}

func (h *WebchatHandler) CreateThread(w http.ResponseWriter, r *http.Request) {
	out, err := h.UC.CreateThread(r.Context(), adminUserID(r))
	if err != nil {
		writeErr(w, r, "webchat", err)
		return
	}
	presenter.WriteJSON(w, http.StatusCreated, presenter.CreateThread(out))
}

func (h *WebchatHandler) GetThread(w http.ResponseWriter, r *http.Request) {
	tid, err := parseThreadID(r.PathValue("threadID"))
	if err != nil {
		writeErr(w, r, "webchat", err)
		return
	}
	after, _ := strconv.ParseUint(r.URL.Query().Get("after_seq"), 10, 64)
	snap, err := h.UC.GetThread(r.Context(), tid, after)
	if err != nil {
		writeErr(w, r, "webchat", err)
		return
	}
	presenter.WriteJSON(w, http.StatusOK, presenter.ThreadSnapshot(snap))
}

func (h *WebchatHandler) RenameThread(w http.ResponseWriter, r *http.Request) {
	tid, err := parseThreadID(r.PathValue("threadID"))
	if err != nil {
		writeErr(w, r, "webchat", err)
		return
	}
	var body struct {
		Title string `json:"title"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, "webchat", err)
		return
	}
	title, err := valueobject.NewTitle(body.Title)
	if err != nil {
		writeValidation(w, r, "title required (max 60)")
		return
	}
	out, err := h.UC.RenameThread(r.Context(), tid, title)
	if err != nil {
		writeErr(w, r, "webchat", err)
		return
	}
	presenter.WriteJSON(w, http.StatusOK, presenter.RenameThread(out))
}

func (h *WebchatHandler) StartTurn(w http.ResponseWriter, r *http.Request) {
	tid, err := parseThreadID(r.PathValue("threadID"))
	if err != nil {
		writeErr(w, r, "webchat", err)
		return
	}
	var body struct {
		Message       string   `json:"message"`
		AttachmentIDs []string `json:"attachment_ids"`
		Model         string   `json:"model"`
		Effort        string   `json:"effort"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, "webchat", err)
		return
	}
	if body.Message == "" && len(body.AttachmentIDs) == 0 {
		writeValidation(w, r, "message empty")
		return
	}
	out, err := h.UC.StartTurn(r.Context(), webchat.StartTurnInput{
		ThreadID:      tid,
		Message:       body.Message,
		AdminUserID:   adminUserID(r),
		AdminName:     adminDisplayName(r),
		AttachmentIDs: body.AttachmentIDs,
		Model:         body.Model,
		Effort:        body.Effort,
	})
	if err != nil {
		writeErr(w, r, "webchat", err)
		return
	}
	presenter.WriteJSON(w, http.StatusAccepted, presenter.StartTurn(out))
}

func (h *WebchatHandler) UploadAttachment(w http.ResponseWriter, r *http.Request) {
	tid, err := parseThreadID(r.PathValue("threadID"))
	if err != nil {
		writeErr(w, r, "webchat", err)
		return
	}
	const maxMem = 10 << 20
	if err := r.ParseMultipartForm(maxMem); err != nil {
		writeValidation(w, r, "multipart required")
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		writeValidation(w, r, "file required")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxMem+1))
	if err != nil {
		writeErr(w, r, "webchat.upload", apperr.New(http.StatusInternalServerError, apperr.CodeInternal, "read failed"))
		return
	}
	if int64(len(data)) > maxMem {
		writeValidation(w, r, "file too large")
		return
	}
	mime := hdr.Header.Get("Content-Type")
	meta, err := h.UC.UploadAttachment(r.Context(), webchat.UploadAttachmentInput{
		ThreadID:    tid,
		Filename:    hdr.Filename,
		Mime:        mime,
		Data:        data,
		AdminUserID: adminUserID(r),
	})
	if err != nil {
		writeErr(w, r, "webchat", err)
		return
	}
	presenter.WriteJSON(w, http.StatusCreated, presenter.Attachment(meta))
}

func (h *WebchatHandler) ListAttachments(w http.ResponseWriter, r *http.Request) {
	tid, err := parseThreadID(r.PathValue("threadID"))
	if err != nil {
		writeErr(w, r, "webchat", err)
		return
	}
	list, err := h.UC.ListAttachments(r.Context(), tid)
	if err != nil {
		writeErr(w, r, "webchat", err)
		return
	}
	presenter.WriteJSON(w, http.StatusOK, presenter.AttachmentList(list))
}

func (h *WebchatHandler) RetryTurn(w http.ResponseWriter, r *http.Request) {
	tid, err := parseThreadID(r.PathValue("threadID"))
	if err != nil {
		writeErr(w, r, "webchat", err)
		return
	}
	var body struct {
		TurnID string `json:"turn_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, "webchat", err)
		return
	}
	turnID, err := parseTurnID(body.TurnID)
	if err != nil {
		writeErr(w, r, "webchat", err)
		return
	}
	out, err := h.UC.RetryTurn(r.Context(), tid, turnID, adminUserID(r))
	if err != nil {
		writeErr(w, r, "webchat", err)
		return
	}
	presenter.WriteJSON(w, http.StatusAccepted, presenter.StartTurn(out))
}

func (h *WebchatHandler) InterruptTurn(w http.ResponseWriter, r *http.Request) {
	tid, err := parseThreadID(r.PathValue("threadID"))
	if err != nil {
		writeErr(w, r, "webchat", err)
		return
	}
	var body struct {
		TurnID string `json:"turn_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, "webchat", err)
		return
	}
	turnID, err := parseTurnID(body.TurnID)
	if err != nil {
		writeErr(w, r, "webchat", err)
		return
	}
	if err := h.UC.InterruptTurn(r.Context(), tid, turnID, adminUserID(r)); err != nil {
		writeErr(w, r, "webchat", err)
		return
	}
	presenter.WriteJSON(w, http.StatusAccepted, map[string]any{
		"ok":     true,
		"status": "interrupt_requested",
	})
}

func (h *WebchatHandler) Events(w http.ResponseWriter, r *http.Request) {
	tid, err := parseThreadID(r.PathValue("threadID"))
	if err != nil {
		writeErr(w, r, "webchat", err)
		return
	}
	afterSeq := parseAfterSeq(r)

	flusher, ok := presenter.WriteSSEHeaders(w)
	if !ok {
		writeErr(w, r, "webchat.events", apperr.New(http.StatusInternalServerError, apperr.CodeInternal, "streaming unsupported"))
		return
	}

	ctx := r.Context()
	pingEvery := 15 * time.Second
	ping := time.NewTicker(pingEvery)
	defer ping.Stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- h.UC.SubscribeEvents(ctx, tid, afterSeq, func(eventName string, payload map[string]any) error {
			var seq uint64
			if v, ok := payload["seq"]; ok {
				switch n := v.(type) {
				case uint64:
					seq = n
				case float64:
					seq = uint64(n)
				case int:
					seq = uint64(n)
				case int64:
					seq = uint64(n)
				}
			}
			return presenter.WriteSSEEvent(w, flusher, eventName, seq, payload)
		})
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case err := <-errCh:
			if err != nil && ctx.Err() == nil {
				logging.Error(ctx, "webchat.events", err)
				// Stream already opened — cannot switch to JSON error.
				_ = presenter.WriteSSEEvent(w, flusher, "turn.failed", 0, map[string]any{
					"type":  "turn.failed",
					"error": map[string]any{"code": "internal", "message": err.Error()},
				})
			}
			return
		case <-ping.C:
			if err := presenter.WriteSSEComment(w, flusher, "ping"); err != nil {
				return
			}
		}
	}
}

func parseAfterSeq(r *http.Request) uint64 {
	after, _ := strconv.ParseUint(r.URL.Query().Get("after_seq"), 10, 64)
	if last := r.Header.Get("Last-Event-ID"); last != "" {
		if n, err := strconv.ParseUint(last, 10, 64); err == nil && n > after {
			after = n
		}
	}
	return after
}

func parseThreadID(raw string) (valueobject.ThreadID, error) {
	id, err := valueobject.NewThreadID(raw)
	if err != nil {
		return "", apperr.New(http.StatusUnprocessableEntity, apperr.CodeValidation, "thread_id required")
	}
	return id, nil
}

func parseTurnID(raw string) (valueobject.TurnID, error) {
	id, err := valueobject.NewTurnID(raw)
	if err != nil {
		return "", apperr.New(http.StatusUnprocessableEntity, apperr.CodeValidation, "turn_id required")
	}
	return id, nil
}
