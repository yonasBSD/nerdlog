//go:build !darwin && !linux && !windows && cgo
// +build !darwin,!linux,!windows,cgo

package clipboard

import (
	"github.com/juju/errors"
)

var InitErr = errors.New("clipboard is only supported on Linux, MacOS and Windows")

// WriteText is a wrapper around clipboard.Write with FmtText; it exists so
// that we can avoid compiling it on unsupported platforms (e.g. FreeBSD) and
// still have nerdlog working (without clipboard support).
func WriteText(value []byte) {
	// no-op
}
