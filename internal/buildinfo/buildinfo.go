package buildinfo

import "strconv"

// Version, CommitID, GitDirty and GoVersion are injected at build time via
// -ldflags. The dev defaults deliberately mark an unstamped workspace build
// as dirty so snapshots never claim provenance they do not have.
var (
	Version   = "dev"
	CommitID  = "unknown"
	GitDirty  = "true"
	GoVersion = "unknown"
)

// IsGitDirty reports whether the build workspace had uncommitted changes.
// A malformed injected value falls back to dirty=true.
func IsGitDirty() bool {
	dirty, err := strconv.ParseBool(GitDirty)
	if err != nil {
		return true
	}
	return dirty
}
