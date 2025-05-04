package core

import "io"

// ShellTransport provides an abstraction for getting shell access to a host;
// e.g. via SSH or just local shell. In the future, tsh (Teleport) might be
// supported as well, and maybe something else.
type ShellTransport interface {
	// Connect attempts to connect to the shell. It just spawns a goroutine and
	// returns immediately, and later on the result (or maybe requests for
	// additional data such as passphrases) will be delivered to the provided
	// channel.
	Connect(resCh chan<- ShellConnUpdate)
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

// ShellConnUpdate contains the update from ssh connection. Exactly one
// field must be non-nil.
type ShellConnUpdate struct {
	// DataRequest contains request for additional data from the user.
	DataRequest *ShellConnDataRequest

	// Result contains the final connection result. After receiving an update
	// with non-nil Result, there will be no more messages to this channel.
	Result *ShellConnResult
}

// ShellConnResult contains the connection result. If the connection is
// successful, Conn is non-nil; otherwise, Err is non-nil.
type ShellConnResult struct {
	Conn ShellConn
	Err  error
}

// ShellConnDataRequest contains request for additional data from the user
// which might be needed during connection, e.g. the passphrase to decrypt
// ssh private key.
type ShellConnDataRequest struct {
	// Title is a human-readable title to show on the data request dialog.
	// If empty, will be "Data request".
	Title string

	// Message is a human-readable message to show to the user when asking for data.
	Message string

	// DataKind specifies what kind of data we're requesting from the user.
	DataKind ShellConnDataKind

	// ResponseCh is where the user response should be sent.
	ResponseCh chan<- string
}

type ShellConnDataKind int

const (
	ShellConnDataKindPassword ShellConnDataKind = iota
)
