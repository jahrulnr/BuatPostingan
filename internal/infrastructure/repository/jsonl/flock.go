package jsonl

import (
	"os"
	"path/filepath"
)

func withFileLock(path string, fn func(f *os.File) error) error {
	if err := os.MkdirAll(dirOf(path), 0o775); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o664)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := flockExclusive(f); err != nil {
		return err
	}
	defer flockUnlock(f)
	return fn(f)
}

// dirOf returns the directory portion of path. Uses filepath.Dir so the
// OS-native separator is honored (forward slash on unix, backslash on windows).
func dirOf(path string) string {
	return filepath.Dir(path)
}
