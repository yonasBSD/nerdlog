package core

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/juju/errors"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const connectionTimeout = 5 * time.Second

type HostAgent struct {
	params HostAgentParams

	connectResCh chan hostConnectRes

	state HostAgentState

	conn *connCtx
}

type connCtx struct {
	sshClient  *ssh.Client
	sshSession *ssh.Session
	stdinBuf   io.WriteCloser

	stdoutLinesCh chan string
	stderrLinesCh chan string
}

func (c *connCtx) getStdoutLinesCh() chan string {
	if c == nil {
		return nil
	}

	return c.stdoutLinesCh
}

func (c *connCtx) getStderrLinesCh() chan string {
	if c == nil {
		return nil
	}

	return c.stderrLinesCh
}

// HostAgentUpdate represents an update from host agent. Name is always
// populated and it's the host's name, and from all the other fields, exactly
// one field must be non-nil.
type HostAgentUpdate struct {
	Name string

	State *HostAgentUpdateState
}

type HostAgentUpdateState struct {
	OldState HostAgentState
	NewState HostAgentState
}

type HostAgentParams struct {
	Config ConfigHost

	UpdatesCh chan<- *HostAgentUpdate
}

func NewHostAgent(params HostAgentParams) *HostAgent {
	ha := &HostAgent{
		params: params,

		state: HostAgentStateDisconnected,
	}

	ha.changeState(HostAgentStateConnecting)

	go ha.run()

	return ha
}

func (ha *HostAgent) SendFoo() {
}

type HostAgentState string

const (
	HostAgentStateDisconnected  HostAgentState = "disconnected"
	HostAgentStateConnecting    HostAgentState = "connecting"
	HostAgentStateConnectedIdle HostAgentState = "connected_idle"
	HostAgentStateConnectedBusy HostAgentState = "connected_busy"
)

func isStateConnected(state HostAgentState) bool {
	return state == HostAgentStateConnectedIdle || state == HostAgentStateConnectedBusy
}

func (ha *HostAgent) changeState(newState HostAgentState) {
	oldState := ha.state

	// Properly leave old state

	if isStateConnected(oldState) && !isStateConnected(newState) {
		ha.conn.sshClient.Close()
		ha.conn.sshSession.Close()
		ha.conn.stdinBuf.Close()
		ha.conn = nil
	}

	switch oldState {
	case HostAgentStateConnecting:
		ha.connectResCh = nil
	}

	// Enter new state

	ha.state = newState
	ha.sendUpdate(&HostAgentUpdate{
		State: &HostAgentUpdateState{
			OldState: oldState,
			NewState: newState,
		},
	})

	switch ha.state {
	case HostAgentStateConnecting:
		ha.connectResCh = make(chan hostConnectRes, 1)
		go connectToHost(ha.params.Config, ha.connectResCh)
	}
}

func (ha *HostAgent) run() {
	for {
		select {
		case res := <-ha.connectResCh:
			if res.err != nil {
				// TODO: backoff
				ha.changeState(HostAgentStateDisconnected)
				ha.changeState(HostAgentStateConnecting)
				continue
			}

			ha.conn = res.conn
			ha.changeState(HostAgentStateConnectedIdle)

		case line := <-ha.conn.getStdoutLinesCh():
			// TODO: depending on state
			_ = line
			//fmt.Println("rx:", line)

		case line := <-ha.conn.getStderrLinesCh():
			// TODO maybe save somewhere for debugging
			fmt.Println("rxe:", line)
			_ = line

			//case data := <-ha.stdinCh:
			//ha.stdinBuf.Write([]byte(data))
			//if len(data) > 0 && data[len(data)-1] != '\n' {
			//ha.stdinBuf.Write([]byte("\n"))
			//}
		}
	}
}

func (ha *HostAgent) sendUpdate(upd *HostAgentUpdate) {
	upd.Name = ha.params.Config.Name
	ha.params.UpdatesCh <- upd
}

type hostConnectRes struct {
	conn *connCtx
	err  error
}

func connectToHost(
	config ConfigHost,
	resCh chan<- hostConnectRes,
) (res hostConnectRes) {
	defer func() {
		resCh <- res
	}()

	jumphost, err := getJumphostClient()
	if err != nil {
		res.err = errors.Annotatef(err, "getting jumphost client")
		return res
	}

	hostAddr := fmt.Sprintf("%s:%d", config.Hostname, config.Port)

	conn, err := dialWithTimeout(jumphost, "tcp", hostAddr, connectionTimeout)
	if err != nil {
		res.err = errors.Trace(err)
		return res
	}

	conf := getClientConfig(config.User)
	//fmt.Printf("hey %+v %q\n", config, hostAddr)
	//fmt.Println("hey2", conn, conf)

	authConn, chans, reqs, err := ssh.NewClientConn(conn, hostAddr, conf)
	if err != nil {
		res.err = errors.Trace(err)
		return res
	}

	sshClient := ssh.NewClient(authConn, chans, reqs)
	//defer client.Close()

	//fmt.Println("sshClient ok", sshClient)

	sshSession, err := sshClient.NewSession()
	if err != nil {
		res.err = errors.Trace(err)
		return res
	}

	//defer sess.Close()

	//fmt.Println("sshSession ok", sshSession)

	stdinBuf, err := sshSession.StdinPipe()
	if err != nil {
		res.err = errors.Trace(err)
		return res
	}
	//fmt.Println("stdin ok")

	stdoutBuf, err := sshSession.StdoutPipe()
	if err != nil {
		res.err = errors.Trace(err)
		return res
	}
	//fmt.Println("stdout ok")

	stderrBuf, err := sshSession.StderrPipe()
	if err != nil {
		res.err = errors.Trace(err)
		return res
	}
	//fmt.Println("stderr ok")

	err = sshSession.Shell()
	if err != nil {
		res.err = errors.Trace(err)
		return res
	}
	//fmt.Println("shell ok")

	stdoutLinesCh := make(chan string, 32)
	stderrLinesCh := make(chan string, 32)

	go getScannerFunc("stdout", stdoutBuf, stdoutLinesCh)()
	go getScannerFunc("stderr", stderrBuf, stderrLinesCh)()

	res.conn = &connCtx{
		sshClient:  sshClient,
		sshSession: sshSession,
		stdinBuf:   stdinBuf,

		stdoutLinesCh: stdoutLinesCh,
		stderrLinesCh: stderrLinesCh,
	}

	return res
}

func getClientConfig(username string) *ssh.ClientConfig {
	auth, err := getSSHAgentAuth()
	if err != nil {
		panic(err.Error())
	}

	return &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{auth},
		// TODO FIX
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
		Timeout: connectionTimeout,
	}
}

var (
	jumphostShared    *ssh.Client
	jumphostSharedMtx sync.Mutex
)

func getJumphostClient() (*ssh.Client, error) {
	jumphostSharedMtx.Lock()
	defer jumphostSharedMtx.Unlock()

	if jumphostShared == nil {
		fmt.Println("Connecting to jumphost...")
		addrs, err := net.LookupHost("dummyhost.com")
		if err != nil {
			return nil, errors.Trace(err)
		}

		if len(addrs) != 1 {
			return nil, errors.New("Address not found")
		}

		conf := getClientConfig("ubuntu")

		jumphost, err := ssh.Dial("tcp", "dummyhost.com:1234", conf)
		if err != nil {
			return nil, errors.Trace(err)
		}

		fmt.Println("Jumphost ok")
		jumphostShared = jumphost
	}

	return jumphostShared, nil
}

var (
	sshAuthMethodShared    ssh.AuthMethod
	sshAuthMethodSharedMtx sync.Mutex
)

func getSSHAgentAuth() (ssh.AuthMethod, error) {
	sshAuthMethodSharedMtx.Lock()
	defer sshAuthMethodSharedMtx.Unlock()

	if sshAuthMethodShared == nil {
		sshAgent, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
		if err != nil {
			return nil, errors.Trace(err)
		}

		sshAuthMethodShared = ssh.PublicKeysCallback(agent.NewClient(sshAgent).Signers)
	}

	return sshAuthMethodShared, nil
}

// dialWithTimeout is a hack needed to get a timeout for the ssh client.
// https://stackoverflow.com/questions/31554196/ssh-connection-timeout
//
// It's possible we could accomplish the same thing by using NewClient() with Conn.SetDeadline(), but that requires
// some refactoring.
func dialWithTimeout(client *ssh.Client, protocol, hostAddr string, timeout time.Duration) (net.Conn, error) {
	finishedChan := make(chan net.Conn)
	errChan := make(chan error)
	go func() {
		conn, err := client.Dial(protocol, hostAddr)
		if err != nil {
			errChan <- err
			return
		}
		finishedChan <- conn
	}()

	select {
	case conn := <-finishedChan:
		return conn, nil

	case err := <-errChan:
		return nil, errors.Trace(err)

	case <-time.After(connectionTimeout):
		// Don't close the connection here since it's reused
		return nil, errors.New("ssh client dial timed out")
	}
}

func getScannerFunc(name string, reader io.Reader, linesCh chan<- string) func() {
	return func() {
		scanner := bufio.NewScanner(reader)
		// TODO: also defer signal to reconnect

		for scanner.Scan() {
			linesCh <- scanner.Text()
		}

		if err := scanner.Err(); err != nil {
			fmt.Println("stdin read error", err)
			return
		} else {
			fmt.Println("stdin EOF")
		}

		fmt.Println("stopped reading stdin")
	}
}
