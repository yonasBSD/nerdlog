package version

import (
	"fmt"
	"strings"
)

// These are being replaced with the actual values using ldflags;
// see ../.goreleaser.yaml and ../Makefile.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	builtBy = "unknown"
)

// VersionFullDescr returns the full version description, printed at
// --version and :version
func VersionFullDescr() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Nerdlog %s\n", version))
	sb.WriteString(fmt.Sprintf("Commit: %s\n", commit))
	sb.WriteString(fmt.Sprintf("Build time: %s\n", date))
	sb.WriteString(fmt.Sprintf("Built by: %s\n", builtBy))
	if cgoEnabled {
		sb.WriteString("CGO: enabled\n")
	} else {
		sb.WriteString("CGO: disabled\n")
	}
	sb.WriteString("\n")
	sb.WriteString("Written by Dmitry Frank (https://dmitryfrank.com)\n")

	return sb.String()
}
