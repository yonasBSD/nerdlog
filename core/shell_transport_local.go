package core

import (
	"io"
	"os/exec"

	"github.com/dimonomid/nerdlog/log"
	"github.com/juju/errors"
)

// ShellTransportLocal is an implementation of ShellTransport that opens a
// local shell.
type ShellTransportLocal struct {
	params ShellTransportLocalParams
}

type ShellTransportLocalParams struct {
	Logger *log.Logger
}

// NewShellTransportLocal creates a new ShellTransportLocal with the given shell command.
func NewShellTransportLocal(params ShellTransportLocalParams) *ShellTransportLocal {
	return &ShellTransportLocal{
		params: params,
	}
}

// Connect starts the local shell and sends the result to the provided channel.
func (s *ShellTransportLocal) Connect(resCh chan<- ShellConnUpdate) {
	go s.doConnect(resCh)
}

func (s *ShellTransportLocal) doConnect(
	resCh chan<- ShellConnUpdate,
) (res ShellConnResult) {
	logger := s.params.Logger

	defer func() {
		if res.Err != nil {
			logger.Errorf("Connection failed: %s", res.Err)
		}

		resCh <- ShellConnUpdate{
			Result: &res,
		}
	}()

	// Start the local shell command
	// TODO: make it configurable; e.g. it likely won't work on Windows
	cmd := exec.Command("/usr/bin/env", "bash")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		res.Err = errors.Annotatef(err, "getting stdin pipe")
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		res.Err = errors.Annotatef(err, "getting stdout pipe")
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		res.Err = errors.Annotatef(err, "getting stderr pipe")
	}

	// Start the shell command
	err = cmd.Start()
	if err != nil {
		res.Err = errors.Annotatef(err, "starting shell")
	}

	res.Conn = &ShellConnLocal{stdin: stdin, stdout: stdout, stderr: stderr, cmd: cmd}

	return res
}

type ShellConnLocal struct {
	cmd *exec.Cmd

	stdin  io.WriteCloser
	stdout io.Reader
	stderr io.Reader
}

func (s *ShellConnLocal) Stdin() io.Writer {
	return s.stdin
}

func (s *ShellConnLocal) Stdout() io.Reader {
	return s.stdout
}

func (s *ShellConnLocal) Stderr() io.Reader {
	return s.stderr
}

func (s *ShellConnLocal) Close() {
	s.stdin.Close()
}
