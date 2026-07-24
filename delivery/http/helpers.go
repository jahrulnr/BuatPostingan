package httpdelivery

import (
	"encoding/json"
	"net/http"
	"strconv"

	"buatpostingan/delivery/presenter"
	"buatpostingan/internal/pkg/apperr"
	"buatpostingan/internal/pkg/logging"
)

// writeErr logs 5xx/unknown errors at the HTTP boundary, then writes the JSON error body.
func writeErr(w http.ResponseWriter, r *http.Request, op string, err error) {
	logging.BoundaryHTTPError(r.Context(), op, err)
	presenter.WriteAppError(w, err)
}

func writeValidation(w http.ResponseWriter, r *http.Request, message string) {
	writeErr(w, r, "http.validation", apperr.New(http.StatusUnprocessableEntity, apperr.CodeValidation, message))
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		return apperr.New(http.StatusUnprocessableEntity, apperr.CodeValidation, "invalid json body")
	}
	return nil
}

// Demo auth stub — replace with real session/guard later.
func adminUserID(r *http.Request) int64 {
	if v := r.Header.Get("X-Admin-User-Id"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return 1
}

func adminDisplayName(r *http.Request) string {
	if v := r.Header.Get("X-Admin-Display-Name"); v != "" {
		return v
	}
	return "Admin User"
}
