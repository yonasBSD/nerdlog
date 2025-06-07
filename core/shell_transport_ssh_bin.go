package core

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/dimonomid/nerdlog/log"
	"github.com/juju/errors"
)

const echoMarkerConnected = "__CONNECTED__"

// ShellTransportSSHBin is an implementation of ShellTransport that opens an
// ssh session using external ssh binary.
type ShellTransportSSHBin struct {
	params ShellTransportSSHBinParams
}

type ShellTransportSSHBinParams struct {
	Host string
	User string
	Port string

	Logger *log.Logger
}

// NewShellTransportSSHBin creates a new ShellTransportSSHBin with the given shell command.
func NewShellTransportSSHBin(params ShellTransportSSHBinParams) *ShellTransportSSHBin {
	params.Logger = params.Logger.WithNamespaceAppended("TransportSSHBin")

	return &ShellTransportSSHBin{
		params: params,
	}
}

// Connect starts the local shell and sends the result to the provided channel.
func (s *ShellTransportSSHBin) Connect(resCh chan<- ShellConnUpdate) {
	go s.doConnect(resCh)
}

func (s *ShellTransportSSHBin) doConnect(
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

	var sshArgs []string
	if s.params.Port != "" {
		sshArgs = append(sshArgs, "-p", s.params.Port)
	}

	// We can't easily intercept any prompts for passwords etc, because ssh
	// interacts directly with the terminal, not stdin/stdout, and so we don't
	// even try, and instruct ssh to fail instead of prompting.
	//
	// We might, in theory, use a PTY like https://github.com/creack/pty, but
	// it's not gonna be very robust and smells like it might open a can of
	// worms, so not for now.
	sshArgs = append(sshArgs, "-o", "BatchMode=yes")

	dest := s.params.Host
	if s.params.User != "" {
		dest = fmt.Sprintf("%s@%s", s.params.User, dest)
	}
	sshArgs = append(sshArgs, dest, "/bin/sh")

	var sshCmdDebugBuilder strings.Builder
	sshCmdDebugBuilder.WriteString("ssh")
	for _, v := range sshArgs {
		sshCmdDebugBuilder.WriteString(" ")
		sshCmdDebugBuilder.WriteString(shellQuote(v))
	}
	sshCmdDebug := sshCmdDebugBuilder.String()

	resCh <- ShellConnUpdate{
		DebugInfo: s.makeDebugInfo(fmt.Sprintf(
			"Trying to connect using external command: %q", sshCmdDebug,
		)),
	}
	logger.Infof("Executing external ssh command: %q", sshCmdDebug)

	cmd := exec.Command("ssh", sshArgs...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		res.Err = errors.Annotatef(err, "getting stdin pipe")
		return res
	}
	rawStdout, err := cmd.StdoutPipe()
	if err != nil {
		res.Err = errors.Annotatef(err, "getting stdout pipe")
		return res
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		res.Err = errors.Annotatef(err, "getting stderr pipe")
		return res
	}

	if err := cmd.Start(); err != nil {
		res.Err = errors.Annotatef(err, "starting shell")
		return res
	}

	// To make sure we were able to connect, we just write "echo __CONNECTED__"
	// to stdin, and wait for it to show up in the stdout.

	resCh <- ShellConnUpdate{
		DebugInfo: s.makeDebugInfo(fmt.Sprintf(
			"Command started, writing \"echo %s\", waiting for it in stdout", echoMarkerConnected,
		)),
	}

	_, err = fmt.Fprintf(stdin, "echo %s\n", echoMarkerConnected)
	if err != nil {
		res.Err = errors.Annotatef(err, "writing connection marker")
		return res
	}

	clientStdoutR, clientStdoutW := io.Pipe()
	scanner := bufio.NewScanner(rawStdout)
	connErrCh := make(chan error)
	go func() {
		defer clientStdoutW.Close()
		for scanner.Scan() {
			line := scanner.Text()
			logger.Verbose3f("Got line while looking for connected marker: %s", line)
			if line == echoMarkerConnected {
				logger.Verbose3f("Got the marker, switching to raw passthrough for stdout")
				// Done waiting, switch to raw passthrough
				connErrCh <- nil
				io.Copy(clientStdoutW, rawStdout)
				return
			}
		}
		if err := scanner.Err(); err != nil {
			logger.Errorf("Got scanner error while waiting for connection marker: %s", err.Error())
			connErrCh <- errors.Annotatef(err, "reading from stdout while waiting for connection marker")
		} else {
			// Got EOF while waiting for the marker; apparently ssh failed to connect,
			// so just read up all stderr (which likely contains the actual error message),
			// and return it as an error.
			stderrBytes, _ := io.ReadAll(stderr)
			connErrCh <- errors.Errorf(
				"failed to connect using external command \"%s\": %s",
				sshCmdDebug, string(stderrBytes),
			)
		}
	}()

	// Wait for the marker to show up in output.
	select {
	case err := <-connErrCh:
		if err != nil {
			res.Err = errors.Trace(err)
			return res
		}

		resCh <- ShellConnUpdate{
			DebugInfo: s.makeDebugInfo("Got the marker, connected successfully"),
		}

		// Got the marker, so we're done.
		res.Conn = &ShellConnSSHBin{
			cmd:    cmd,
			stdin:  stdin,
			stdout: clientStdoutR,
			stderr: stderr,
		}
		return res

	case <-time.After(connectionTimeout):
		res.Err = errors.New("timeout waiting for SSH connection marker")
		return res
	}
}

func (s *ShellTransportSSHBin) makeDebugInfo(message string) *ShellConnDebugInfo {
	return &ShellConnDebugInfo{
		Message: message,
	}
}

type ShellConnSSHBin struct {
	cmd *exec.Cmd

	stdin  io.WriteCloser
	stdout io.Reader
	stderr io.Reader
}

func (s *ShellConnSSHBin) Stdin() io.Writer {
	return s.stdin
}

func (s *ShellConnSSHBin) Stdout() io.Reader {
	return s.stdout
}

func (s *ShellConnSSHBin) Stderr() io.Reader {
	return s.stderr
}

func (s *ShellConnSSHBin) Close() {
	s.stdin.Close()
}
