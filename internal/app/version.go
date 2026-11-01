package app

import (
	"fmt"
	"runtime"
)

// These values can be replaced with -ldflags at release time.
var (
	Version   = "0.1.0-p01"
	Commit    = "unknown"
	BuildDate = "unknown"
)

func VersionString() string {
	return fmt.Sprintf("livecheck %s (commit %s, built %s, %s)", Version, Commit, BuildDate, runtime.Version())
}
