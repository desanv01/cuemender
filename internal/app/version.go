package app

import (
	"fmt"
	"runtime"
)

// These values can be replaced with -ldflags at release time.
var (
	Version   = "0.4.0-p04"
	Commit    = "unknown"
	BuildDate = "unknown"
)

func VersionString() string {
	return fmt.Sprintf("cuemender %s (commit %s, built %s, %s)", Version, Commit, BuildDate, runtime.Version())
}
