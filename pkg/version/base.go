// Copyright Contributors to the Open Cluster Management project

package version

// Base version information.
//
// This is the fallback data used when version information from git is not
// provided via go ldflags (via Makefile).
var (
	// Output of "git describe". The prerequisite is that the branch should be
	// tagged using the correct versioning strategy.
	gitVersion = "v0.0.1-alpha.0"
	// SHA1 from git, output of $(git rev-parse HEAD)
	gitCommit = "unknown"
	// State of git tree, either "clean" or "dirty"
	gitTreeState = "unknown"
	// Build date in ISO8601 format, output of $(date -u +'%Y-%m-%dT%H:%M:%SZ')
	buildDate = "unknown"
)
