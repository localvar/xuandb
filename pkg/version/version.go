package version

import (
	"runtime"
	"runtime/debug"
)

// version is the version number of the application, set at build time.
var version string

// Version returns version number of the application.
func Version() string {
	return version
}

// GoVersion returns the version of Go used to build the application.
func GoVersion() string {
	return runtime.Version()
}

func readBuildSetting(key string) string {
	bi, _ := debug.ReadBuildInfo()
	if bi == nil {
		return ""
	}
	for _, bs := range bi.Settings {
		if bs.Key == key {
			return bs.Value
		}
	}
	return ""
}

// Revision returns the VCS revision/commit id of the code which the
// application is build from.
func Revision() string {
	return readBuildSetting("vcs.revision")
}

// LocalModified returns whether the code contains local modifications, i.e.
// uncommitted changes.
func LocalModified() bool {
	return readBuildSetting("vcs.modified") == "true"
}
