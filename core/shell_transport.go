package core

import "io"

// ShellTransport provides an abstraction for getting shell access to a host;
// e.g. via SSH or just local shell. In the future, tsh (Teleport) might be
// supported as well, and maybe something else.
type ShellTransport interface {
	// Connect attempts to connect to the shell. It just spawns a goroutine and
	// returns immediately, and later on the result will be delivered to the
	// provided channel. Exactly 1 message will be sent to this channel.
	Connect(resCh chan<- ShellConnResult)
}

// ShellConn provides an abstraction of a shell connection; can be implemented
// by local shell, or SSH, or maybe something else.
type ShellConn interface {
	Stdin() io.Writer
	Stdout() io.Reader
	Stderr() io.Reader

	// Close should be called when the connection is not needed anymore.
	Close()
}

// ShellConnResult contains the connection result. If the connection is
// successful, Conn is non-nil; otherwise, Err is non-nil.
type ShellConnResult struct {
	Conn ShellConn
	Err  error
}
