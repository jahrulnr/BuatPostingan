package presenter

import (
	"encoding/json"
	"net/http"

	"buatpostingan/internal/pkg/apperr"
)

// ErrorBody matches the FE real-driver expectation.
type ErrorBody struct {
	Error   string         `json:"error"`
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Extra   map[string]any `json:"-"`
}

func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func WriteAppError(w http.ResponseWriter, err error) {
	if ae, ok := apperr.As(err); ok {
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
