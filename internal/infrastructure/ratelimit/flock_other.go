//go:build !unix

package ratelimit

import "os"

// flockExclusive is a no-op on platforms without flock (e.g. Windows).
// Rate-limit file writes are short atomic read-modify-write cycles; in-process
// mutex + atomic truncate/write is sufficient since the limiter runs in-process.
func flockExclusive(f *os.File) error { return nil }

func flockUnlock(f *os.File) error { return nil }
