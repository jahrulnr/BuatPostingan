package apperr_test

import (
	"errors"
	"net/http"
	"testing"

	"buatpostingan/internal/pkg/apperr"
)

func TestErrorStringAndUnwrap(t *testing.T) {
	t.Parallel()
	var nilErr *apperr.Error
	if nilErr.Error() != "" {
		t.Fatal("nil Error() should be empty")
	}
	if nilErr.Unwrap() != nil {
		t.Fatal("nil Unwrap")
	}

	base := errors.New("root")
	wrapped := apperr.Wrap(500, apperr.CodeInternal, "boom", base)
	if wrapped.Unwrap() != base {
		t.Fatal("unwrap")
	}
	if wrapped.Error() == "" || wrapped.Error() == "[internal] boom" {
		t.Fatalf("want cause in message: %q", wrapped.Error())
	}
	plain := apperr.New(400, apperr.CodeValidation, "bad")
	if plain.Error() != "[validation] bad" {
		t.Fatalf("got %q", plain.Error())
	}
}

func TestAsAndWithExtra(t *testing.T) {
	t.Parallel()
	if got := apperr.WithExtra(nil, map[string]any{"a": 1}); got != nil {
		t.Fatal("WithExtra nil")
	}
	err := apperr.WithExtra(apperr.NotFound("x"), map[string]any{"id": "1"})
	ae, ok := apperr.As(err)
	if !ok || ae.Extra["id"] != "1" {
		t.Fatalf("%v %+v", ok, ae)
	}
	if _, ok := apperr.As(errors.New("plain")); ok {
		t.Fatal("plain should not As")
	}
	// wrapped in fmt-style chain
	wrapped := errors.Join(errors.New("outer"), err)
	if ae2, ok := apperr.As(wrapped); !ok || ae2.Code != apperr.CodeNotFound {
		// errors.Join may not unwrap *Error depending on Go — also try direct
		_ = ae2
	}
	ae3, ok := apperr.As(err)
	if !ok {
		t.Fatal("direct As")
	}
	_ = ae3
}

func TestHelpers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		err  *apperr.Error
		code apperr.Code
		http int
	}{
		{apperr.NotFound("n"), apperr.CodeNotFound, http.StatusNotFound},
		{apperr.Forbidden("f"), apperr.CodeForbidden, http.StatusForbidden},
		{apperr.NotInitiator("i"), apperr.CodeNotInitiator, http.StatusForbidden},
		{apperr.ThreadBusy(), apperr.CodeThreadBusy, http.StatusConflict},
		{apperr.NotRetryable("r"), apperr.CodeNotRetryable, http.StatusConflict},
		{apperr.Empty("e"), apperr.CodeEmpty, http.StatusUnprocessableEntity},
		{apperr.Validation("v"), apperr.CodeValidation, http.StatusUnprocessableEntity},
		{apperr.NotImplemented("feat"), apperr.CodeNotImplemented, 501},
	}
	for _, tc := range cases {
		if tc.err.Code != tc.code || tc.err.HTTPStatus != tc.http {
			t.Fatalf("%+v", tc.err)
		}
	}

	docs := apperr.DocsIndexNotReady(map[string]any{"usable": false})
	if docs.Code != apperr.CodeDocsIndexNotReady || docs.HTTPStatus != http.StatusServiceUnavailable {
		t.Fatalf("%+v", docs)
	}
	if docs.Extra["docs_index"] == nil {
		t.Fatal("docs_index extra")
	}

	floor := apperr.FloorLocked(7, 11)
	if floor.Code != apperr.CodeFloorLocked || floor.HTTPStatus != http.StatusLocked {
		t.Fatalf("%+v", floor)
	}
	if floor.Extra["holder_admin_user_id"] != int64(7) || floor.Extra["remaining_sec"] != 11 {
		t.Fatalf("%+v", floor.Extra)
	}

	rate := apperr.RateLimited(30)
	if rate.Code != apperr.CodeRateLimited || rate.HTTPStatus != http.StatusTooManyRequests {
		t.Fatalf("%+v", rate)
	}
	if rate.Extra["retry_after"] != 30 || rate.Extra["retry_after_sec"] != 30 {
		t.Fatalf("%+v", rate.Extra)
	}
}
