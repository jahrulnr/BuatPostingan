//go:build !unix

package jsonl

import "os"

// flockExclusive is a no-op on platforms without flock (e.g. Windows).
// Cross-process exclusivity degrades to atomic appends/rename; in-process
// calls are serialized by the caller's storage mutex.
func flockExclusive(f *os.File) error { return nil }

func flockUnlock(f *os.File) error { return nil }
