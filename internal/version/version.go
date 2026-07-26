// Package version holds the application version, injected at build time via
// -ldflags "-X buatpostingan/internal/version.Version=<v>". Defaults to "dev"
// when built without ldflags (e.g. `go run`).
package version

// Version is the application version. Overridden at build time from VERSION
// file contents (see Makefile / deploy/Dockerfile / GitHub Actions).
var Version = "dev"

// Get returns the current application version.
func Get() string { return Version }
