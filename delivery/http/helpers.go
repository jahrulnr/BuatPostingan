package httpdelivery

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"buatpostingan/delivery/presenter"
	"buatpostingan/internal/pkg/apperr"
	"buatpostingan/internal/pkg/logging"
)

// maxJSONBodyBytes caps JSON request bodies at the HTTP boundary (2 MiB).
const maxJSONBodyBytes = 2 << 20

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
	limited := http.MaxBytesReader(nil, r.Body, maxJSONBodyBytes)
	dec := json.NewDecoder(limited)
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return apperr.New(http.StatusRequestEntityTooLarge, apperr.CodeValidation, "request body too large (max 2 MiB)")
		}
		return apperr.New(http.StatusUnprocessableEntity, apperr.CodeValidation, "invalid json body")
	}
	return nil
}

// Demo auth stub — replace with real session/guard later.
func adminUserID(r *http.Request) int64 {
	if user, ok := userFromContext(r.Context()); ok {
		return user.ID
	}
	if v := r.Header.Get("X-Admin-User-Id"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return 1
}

func adminDisplayName(r *http.Request) string {
	if user, ok := userFromContext(r.Context()); ok {
		return user.DisplayName
	}
	if v := r.Header.Get("X-Admin-Display-Name"); v != "" {
		return v
	}
	return "Admin User"
}
