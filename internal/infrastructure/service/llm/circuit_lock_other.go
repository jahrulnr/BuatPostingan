//go:build !unix

package llm

import "os"

// lockFile is a no-op on platforms without flock (e.g. Windows). In-process
// serialization via the circuitStore mutex still applies; cross-process
// exclusivity degrades to atomic temp+rename writes only.
func lockFile(f *os.File) error { return nil }

func unlockFile(f *os.File) error { return nil }
