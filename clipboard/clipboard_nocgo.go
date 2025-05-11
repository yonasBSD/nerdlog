//go:build !cgo
// +build !cgo

package clipboard

import (
	"github.com/juju/errors"
)

var InitErr = errors.New("nerdlog was built with CGO_ENABLED=0")

// WriteText is a wrapper around clipboard.Write with FmtText; it exists so
// that we can avoid compiling it on unsupported platforms (e.g. FreeBSD) and
// still have nerdlog working (without clipboard support).
func WriteText(value []byte) {
	// no-op
}
