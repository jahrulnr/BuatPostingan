package llm

import (
	"fmt"
)

// Error is a provider call failure (transient → failover).
type Error struct {
	Provider  string
	Status    int
	Transient bool
	Msg       string
	Cause     error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("llm[%s]: %s: %v", e.Provider, e.Msg, e.Cause)
	}
	return fmt.Sprintf("llm[%s]: %s", e.Provider, e.Msg)
}

func (e *Error) Unwrap() error { return e.Cause }
