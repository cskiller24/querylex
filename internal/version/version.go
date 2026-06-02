// Package version provides build-time injected version metadata.
// The variables Version, Commit, and BuildDate are set at link time
// via -ldflags -X flags during both local builds (Makefile) and
// release builds (GoReleaser). Without ldflags injection they
// default to "dev", "unknown", and "unknown" respectively.
package version

var (
	// Version is the semantic version of the binary, injected at build time.
	// Defaults to "dev" when built without ldflags.
	Version = "dev"

	// Commit is the git commit hash from which the binary was built.
	// Defaults to "unknown" when built without ldflags.
	Commit = "unknown"

	// BuildDate is the UTC timestamp of the build, injected at build time.
	// Defaults to "unknown" when built without ldflags.
	BuildDate = "unknown"
)
