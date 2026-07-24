package presenter

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"buatpostingan/internal/pkg/apperr"
)

func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func WriteAppError(w http.ResponseWriter, err error) {
	if ae, ok := apperr.As(err); ok {
		if ae.HTTPStatus == http.StatusTooManyRequests {
			if v, ok := ae.Extra["retry_after"]; ok {
				w.Header().Set("Retry-After", fmt.Sprint(v))
			} else if v, ok := ae.Extra["retry_after_sec"]; ok {
				w.Header().Set("Retry-After", fmt.Sprint(v))
			}
		}
		payload := map[string]any{
			"error":   string(ae.Code),
			"code":    string(ae.Code),
			"message": ae.Message,
		}
		for k, v := range ae.Extra {
			payload[k] = v
		}
		WriteJSON(w, ae.HTTPStatus, payload)
		return
	}
	WriteJSON(w, http.StatusInternalServerError, map[string]any{
		"error":   string(apperr.CodeInternal),
		"code":    string(apperr.CodeInternal),
		"message": "internal error",
	})
}

func WriteValidationError(w http.ResponseWriter, message string) {
	WriteAppError(w, apperr.New(http.StatusUnprocessableEntity, apperr.CodeValidation, message))
}

// WriteSSEHeaders prepares a text/event-stream response.
func WriteSSEHeaders(w http.ResponseWriter) (http.Flusher, bool) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher, ok := w.(http.Flusher)
	if ok {
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
	}
	return flusher, ok
}

// WriteSSEEvent writes one SSE frame. seq, when > 0, becomes the SSE id.
func WriteSSEEvent(w http.ResponseWriter, flusher http.Flusher, eventName string, seq uint64, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if seq > 0 {
		if _, err := fmt.Fprintf(w, "id: %s\n", strconv.FormatUint(seq, 10)); err != nil {
			return err
		}
	}
	if eventName != "" {
		if _, err := fmt.Fprintf(w, "event: %s\n", eventName); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
		return err
	}
	if flusher != nil {
		flusher.Flush()
	}
	return nil
}

func WriteSSEComment(w http.ResponseWriter, flusher http.Flusher, comment string) error {
	if _, err := fmt.Fprintf(w, ": %s\n\n", comment); err != nil {
		return err
	}
	if flusher != nil {
		flusher.Flush()
	}
	return nil
}

func UnixMillis(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}
