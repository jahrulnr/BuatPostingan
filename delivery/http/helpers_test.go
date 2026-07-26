package httpdelivery

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"buatpostingan/internal/pkg/apperr"
)

func TestDecodeJSONRejectsOversizedBody(t *testing.T) {
	huge := `{"username":"` + strings.Repeat("a", maxJSONBodyBytes+1) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(huge))
	var dst map[string]any
	err := decodeJSON(req, &dst)
	if err == nil {
		t.Fatal("expected oversized body error")
	}
	ae, ok := apperr.As(err)
	if !ok || ae.HTTPStatus != http.StatusRequestEntityTooLarge {
		t.Fatalf("err=%v", err)
	}
}
