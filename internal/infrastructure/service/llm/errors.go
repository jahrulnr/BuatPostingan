package llm

import (
	"errors"
	"fmt"
	"time"
)

// Error is a provider call failure (transient → failover).
type Error struct {
	Provider  string
	Status    int
	Transient bool
	Msg       string
	Cause     error
	// RetryAfter is a provider-requested delay parsed from the Retry-After
	// response header (0 when absent). The router caps it by the max backoff.
	RetryAfter time.Duration
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("llm[%s]: %s: %v", e.Provider, e.Msg, e.Cause)
	}
	return fmt.Sprintf("llm[%s]: %s", e.Provider, e.Msg)
}

func (e *Error) Unwrap() error { return e.Cause }

// isSSETransportErr reports Codex-like stream disconnects (incomplete / early close / mid-stream read).
func isSSETransportErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrSSETransport) {
		return true
	}
	var le *Error
	if errors.As(err, &le) && le != nil {
		return errors.Is(le.Cause, ErrSSETransport)
	}
	return false
}

// transientKind classifies a transient error for retry logging (sse | status | connect).
func transientKind(err error) string {
	if isSSETransportErr(err) {
		return "sse_transport"
	}
	var le *Error
	if errors.As(err, &le) && le != nil && le.Status > 0 {
		return "http_status"
	}
	return "connect"
}
