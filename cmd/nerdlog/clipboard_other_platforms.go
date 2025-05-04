//go:build !darwin && !linux && !windows && cgo
// +build !darwin,!linux,!windows,cgo

package main

import (
	"github.com/juju/errors"
)

// clipboardInit is a wrapper around clipboard.Init, it only exists because
// clipboard.Init panics if it was built with CGO_ENABLED=0, but we want just
// an error, not a panic.
func clipboardInit() error {
	return errors.New("clipboard is only supported on Linux, MacOS and Windows")
}

// clipboardWriteText is a wrapper around clipboard.Write with FmtText;
// it exists so that we can avoid compiling it on unsupported platforms
// (e.g. FreeBSD) and still have nerdlog working (without clipboard support).
func clipboardWriteText(value []byte) {
	// no-op
}
