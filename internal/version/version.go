package version

import (
	"fmt"
	"runtime"
)

var (
	version   string
	gitCommit string
	gitBranch string
	buildDate string
)

// Version returns the version at which the binary was built.
func Version() string {
	return version
}

// GitCommit returns the commit hash at which the binary was built.
func GitCommit() string {
	return gitCommit
}

// GitBranch returns the branch at which the binary was built.
func GitBranch() string {
	return gitBranch
}

// BuildDate returns the time at which the binary was built.
func BuildDate() string {
	return buildDate
}

// GoString returns the compiler, compiler version and architecture of the build.
func GoString() string {
	return fmt.Sprintf("%s / %s / %s", runtime.Compiler, runtime.Version(), runtime.GOARCH)
}
