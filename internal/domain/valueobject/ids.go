package valueobject

import (
	"errors"
	"strings"
)

var (
	ErrEmptyThreadID = errors.New("thread_id empty")
	ErrEmptyTurnID   = errors.New("turn_id empty")
	ErrEmptyItemID   = errors.New("item_id empty")
	ErrBadTitle      = errors.New("title invalid")
)

// ThreadID is thr_…
type ThreadID string

func NewThreadID(raw string) (ThreadID, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", ErrEmptyThreadID
	}
	return ThreadID(s), nil
}

func (id ThreadID) String() string { return string(id) }

// TurnID is trn_…
type TurnID string

func NewTurnID(raw string) (TurnID, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", ErrEmptyTurnID
	}
	return TurnID(s), nil
}

func (id TurnID) String() string { return string(id) }

// ItemID is itm_…
type ItemID string

func NewItemID(raw string) (ItemID, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", ErrEmptyItemID
	}
	return ItemID(s), nil
}

func (id ItemID) String() string { return string(id) }

// Title is a conversation title (max 60 runes, trimmed).
type Title string

func NewTitle(raw string) (Title, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", ErrBadTitle
	}
	runes := []rune(s)
	if len(runes) > 60 {
		return "", ErrBadTitle
	}
	return Title(s), nil
}

func (t Title) String() string { return string(t) }
