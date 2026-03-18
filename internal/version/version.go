package version

import (
	"fmt"
	"runtime"
)

var (
	Version   = "dev"     // injected: -X .../version.Version=v1.0.0
	GitCommit = "unknown" // injected: -X .../version.GitCommit=$(git rev-parse --short HEAD)
	BuildDate = "unknown" // injected: -X .../version.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)
)

func GoVersion() string { return runtime.Version() }

// Full returns a multi-line version string for --version flag.
func Full() string {
	return fmt.Sprintf("%s\ncommit: %s\nbuilt:  %s\ngo:     %s", Version, GitCommit, BuildDate, GoVersion())
}

// Short returns just the version string (used in TUI header).
func Short() string { return Version }
