package app

import (
	"fmt"
	"runtime"
)

// These values can be replaced with -ldflags at release time.
var (
	Version   = "0.3.0-p03"
	Commit    = "unknown"
	BuildDate = "unknown"
)

func VersionString() string {
	return fmt.Sprintf("cuemender %s (commit %s, built %s, %s)", Version, Commit, BuildDate, runtime.Version())
}
