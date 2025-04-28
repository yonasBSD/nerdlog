//go:build !cgo

package main

import (
	"github.com/juju/errors"
)

// initClipboard is a wrapper around clipboard.Init, it only exists because
// clipboard.Init panics if it was built with CGO_ENABLED=0, but we want just
// an error, not a panic.
func initClipboard() error {
	return errors.New("nerdlog was built with CGO_ENABLED=0")
}
