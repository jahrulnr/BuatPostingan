//go:build unix

package llm

import (
	"os"
	"syscall"
)

// lockFile takes an advisory exclusive lock (flock) for cross-process circuit
// state updates. Blocks until acquired.
func lockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}

func unlockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
