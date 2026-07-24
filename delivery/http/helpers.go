package httpdelivery

import (
	"encoding/json"
	"net/http"
	"strconv"

	"buatpostingan/internal/pkg/apperr"
)

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
