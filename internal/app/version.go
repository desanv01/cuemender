package app

import (
	"fmt"
	"runtime"
)

// These values can be replaced with -ldflags at release time.
var (
	Version   = "0.5.1-receive-only"
	Commit    = "unknown"
	BuildDate = "unknown"
)

func VersionString() string {
	return fmt.Sprintf("cuemender %s (commit %s, built %s, %s)", Version, Commit, BuildDate, runtime.Version())
}
