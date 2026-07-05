// Package version exposes the build-time version string for Bothan.
package version

// Version is the application version, following the YYYY.M.PATCH scheme.
// It is overridden at build time via
// -ldflags "-X github.com/t0mer/bothan/internal/version.Version=<v>".
var Version = "dev"
