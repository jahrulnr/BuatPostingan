package apperr

import "net/http"

func NotFound(message string) *Error {
	return New(http.StatusNotFound, CodeNotFound, message)
}

func Forbidden(message string) *Error {
	return New(http.StatusForbidden, CodeForbidden, message)
}

func NotInitiator(message string) *Error {
	return New(http.StatusForbidden, CodeNotInitiator, message)
}

func ThreadBusy() *Error {
	return New(http.StatusConflict, CodeThreadBusy, "thread busy")
}

func NotRetryable(message string) *Error {
	return New(http.StatusConflict, CodeNotRetryable, message)
}

func Empty(message string) *Error {
	return New(http.StatusUnprocessableEntity, CodeEmpty, message)
}

func Validation(message string) *Error {
	return New(http.StatusUnprocessableEntity, CodeValidation, message)
}

func DocsIndexNotReady(gate any) *Error {
	return WithExtra(
		New(http.StatusServiceUnavailable, CodeDocsIndexNotReady, "Docs index belum siap"),
		map[string]any{"docs_index": gate},
	)
}

func FloorLocked(holderAdminID int64, remainingSec int) *Error {
	return WithExtra(
		New(http.StatusLocked, CodeFloorLocked, "Speak floor locked"),
		map[string]any{
			"holder_admin_user_id": holderAdminID,
			"remaining_sec":        remainingSec,
		},
	)
}
