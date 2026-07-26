package apperr

import (
	"errors"
	"fmt"
)

// Code is a stable API error code (mirrors FE / contracts).
type Code string

const (
	CodeNotImplemented    Code = "not_implemented"
	CodeThreadNotFound    Code = "thread_not_found"
	CodeNotFound          Code = "not_found"
	CodeThreadBusy        Code = "thread_busy"
	CodeFloorLocked       Code = "floor_locked"
	CodeDocsIndexNotReady Code = "docs_index_not_ready"
	CodeForbidden         Code = "forbidden"
	CodeNotInitiator      Code = "not_initiator"
	CodeValidation        Code = "validation"
	CodeEmpty             Code = "empty"
	CodeNotRetryable      Code = "not_retryable"
	CodeInternal          Code = "internal"
	CodeUpstream          Code = "upstream"
)

// Error is the application boundary error (usecase → delivery).
type Error struct {
	HTTPStatus int
	Code       Code
	Message    string
	Extra      map[string]any
	Cause      error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func New(httpStatus int, code Code, message string) *Error {
	return &Error{HTTPStatus: httpStatus, Code: code, Message: message}
}

func Wrap(httpStatus int, code Code, message string, cause error) *Error {
	return &Error{HTTPStatus: httpStatus, Code: code, Message: message, Cause: cause}
}

func WithExtra(err *Error, extra map[string]any) *Error {
	if err == nil {
		return nil
	}
	err.Extra = extra
	return err
}

func As(err error) (*Error, bool) {
	var target *Error
	if errors.As(err, &target) {
		return target, true
	}
	return nil, false
}

func NotImplemented(feature string) *Error {
	return New(501, CodeNotImplemented, feature+" not implemented yet")
}
