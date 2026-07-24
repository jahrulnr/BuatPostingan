package jsonl

import (
	"context"
	"os"

	"buatpostingan/internal/domain/repository"
	"buatpostingan/internal/domain/valueobject"
)

// Interrupt implements repository.InterruptFlag via flag files.
type Interrupt struct {
	root string
}

var _ repository.InterruptFlag = (*Interrupt)(nil)

func NewInterrupt(storageRoot string) *Interrupt {
	return &Interrupt{root: storageRoot}
}

func (i *Interrupt) Request(ctx context.Context, threadID valueobject.ThreadID, turnID valueobject.TurnID) error {
	_ = ctx
	path := interruptFlagPath(i.root, threadID.String(), turnID.String())
	if err := os.MkdirAll(dirOf(path), 0o775); err != nil {
		return err
	}
	return os.WriteFile(path, []byte("1"), 0o664)
}

func (i *Interrupt) IsRequested(ctx context.Context, threadID valueobject.ThreadID, turnID valueobject.TurnID) (bool, error) {
	_ = ctx
	path := interruptFlagPath(i.root, threadID.String(), turnID.String())
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (i *Interrupt) Clear(ctx context.Context, threadID valueobject.ThreadID, turnID valueobject.TurnID) error {
	_ = ctx
	path := interruptFlagPath(i.root, threadID.String(), turnID.String())
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
