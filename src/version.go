package parallel

import (
	"fmt"
	"runtime"
	"strings"
	"time"
)

// These variables are intended to be set via -ldflags at build time in CI.
// Defaults are for local development builds.
//
//nolint:gochecknoglobals
var (
	Version = "dev"   // e.g. v1.2.3 or git describe
	Commit  = ""      // short SHA
	Date    = ""      // build time, RFC3339
	BuiltBy = "local" // CI system or user
)

const (
	ProjectURL       = "https://github.com/efureev/parallel"
	ProjectDescShort = "Parallel: run chains of commands concurrently with colored structured logs"
)

// VersionShort returns a single-line version string.
func VersionShort() string {
	parts := []string{"parallel", Version}
	if Commit != "" {
		parts = append(parts, fmt.Sprintf("(%s)", Commit))
	}

	return strings.Join(parts, " ")
}

// VersionLong returns a multi-line detailed version info block.
func VersionLong() string {
	bdate := Date
	if bdate == "" {
		bdate = time.Now().UTC().Format(time.RFC3339)
	}

	gover := runtime.Version()
	osarch := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)

	return fmt.Sprintf(`%s
Version:   %s
Commit:    %s
Built at:  %s
Built by:  %s
Go:        %s
OS/Arch:   %s
URL:       %s
`, ProjectDescShort, Version, fallback(Commit, "n/a"), bdate, fallback(BuiltBy, "n/a"), gover, osarch, ProjectURL)
}

func fallback(s, def string) string {
	if s == "" {
		return def
	}

	return s
}
