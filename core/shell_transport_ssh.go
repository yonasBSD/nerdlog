package core

import (
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/dimonomid/nerdlog/log"
	"github.com/juju/errors"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// ShellTransportSSH implements ShellTransport over SSH.
type ShellTransportSSH struct {
	params ShellTransportSSHParams
}

var _ ShellTransport = &ShellTransportSSH{}

func NewShellTransportSSH(params ShellTransportSSHParams) *ShellTransportSSH {
	params.Logger = params.Logger.WithNamespaceAppended("TransportSSH")

	return &ShellTransportSSH{
		params: params,
	}
}

type ShellTransportSSHParams struct {
	ConnDetails ConfigLogStreamShellTransportSSH

	Logger *log.Logger
}

func (st *ShellTransportSSH) Connect(resCh chan<- ShellConnResult) {
	go st.doConnect(resCh)
}

func (st *ShellTransportSSH) doConnect(
	resCh chan<- ShellConnResult,
) (res ShellConnResult) {
	logger := st.params.Logger

	defer func() {
		if res.Err != nil {
			logger.Errorf("Connection failed: %s", res.Err)
		}

		resCh <- res
	}()

	connDetails := st.params.ConnDetails

	var sshClient *ssh.Client

	conf := getClientConfig(logger, connDetails.Host.User)

	if connDetails.Jumphost != nil {
		logger.Infof("Connecting via jumphost")
		// Use jumphost
		jumphost, err := getJumphostClient(logger, connDetails.Jumphost)
		if err != nil {
			logger.Errorf("Jumphost connection failed: %s", err)
			res.Err = errors.Annotatef(err, "getting jumphost client")
			return res
		}

		conn, err := dialWithTimeout(jumphost, "tcp", connDetails.Host.Addr, connectionTimeout)
		if err != nil {
			res.Err = errors.Trace(err)
			return res
		}

		authConn, chans, reqs, err := ssh.NewClientConn(conn, connDetails.Host.Addr, conf)
		if err != nil {
			res.Err = errors.Trace(err)
			return res
		}

		sshClient = ssh.NewClient(authConn, chans, reqs)
	} else {
		logger.Infof("Connecting to %s (%+v)", connDetails.Host.Addr, conf)
		var err error
		sshClient, err = ssh.Dial("tcp", connDetails.Host.Addr, conf)
		if err != nil {
			res.Err = errors.Trace(err)
			return res
		}
	}

	logger.Infof("Connected to %s", connDetails.Host.Addr)

	sshSession, err := sshClient.NewSession()
	if err != nil {
		res.Err = errors.Trace(err)
		return res
	}

	stdinBuf, err := sshSession.StdinPipe()
	if err != nil {
		res.Err = errors.Trace(err)
		return res
	}

	stdoutBuf, err := sshSession.StdoutPipe()
	if err != nil {
		res.Err = errors.Trace(err)
		return res
	}

	stderrBuf, err := sshSession.StderrPipe()
	if err != nil {
		res.Err = errors.Trace(err)
		return res
	}

	err = sshSession.Shell()
	if err != nil {
		res.Err = errors.Trace(err)
		return res
	}

	res.Conn = &ShellConnSSH{
		sshClient:  sshClient,
		sshSession: sshSession,

		stdinBuf:  stdinBuf,
		stdoutBuf: stdoutBuf,
		stderrBuf: stderrBuf,
	}

	return res
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

func getClientConfig(logger *log.Logger, username string) *ssh.ClientConfig {
	auth, err := getSSHAgentAuth(logger)
	if err != nil {
		panic(err.Error())
	}

	return &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{auth},

		// TODO: fix it
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),

		Timeout: connectionTimeout,
	}
}

var (
	sshAuthMethodShared    ssh.AuthMethod
	sshAuthMethodSharedMtx sync.Mutex
)

func getSSHAgentAuth(logger *log.Logger) (ssh.AuthMethod, error) {
	sshAuthMethodSharedMtx.Lock()
	defer sshAuthMethodSharedMtx.Unlock()

	if sshAuthMethodShared == nil {
		logger.Infof("Initializing sshAuthMethodShared...")
		sshAgent, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
		if err != nil {
			logger.Infof("Failed to initialize sshAuthMethodShared: %s", err.Error())
			return nil, errors.Trace(err)
		}

		sshAuthMethodShared = ssh.PublicKeysCallback(agent.NewClient(sshAgent).Signers)
	}

	return sshAuthMethodShared, nil
}

var (
	jumphostsShared    = map[string]*ssh.Client{}
	jumphostsSharedMtx sync.Mutex
)

func getJumphostClient(logger *log.Logger, jhConfig *ConfigHost) (*ssh.Client, error) {
	jumphostsSharedMtx.Lock()
	defer jumphostsSharedMtx.Unlock()

	key := jhConfig.Key()
	jh := jumphostsShared[key]
	if jh == nil {
		logger.Infof("Connecting to jumphost... %+v", jhConfig)

		parts := strings.Split(jhConfig.Addr, ":")
		if len(parts) != 2 {
			return nil, errors.Errorf("malformed jumphost address %q", jhConfig.Addr)
		}

		addrs, err := net.LookupHost(parts[0])
		if err != nil {
			return nil, errors.Trace(err)
		}

		if len(addrs) != 1 {
			return nil, errors.New("Address not found")
		}

		conf := getClientConfig(logger, jhConfig.User)

		jh, err = ssh.Dial("tcp", jhConfig.Addr, conf)
		if err != nil {
			return nil, errors.Trace(err)
		}

		jumphostsShared[key] = jh

		logger.Infof("Jumphost ok")
	}

	return jh, nil
}

// ShellConnSSH implements ShellConn for SSH.
type ShellConnSSH struct {
	sshClient  *ssh.Client
	sshSession *ssh.Session

	stdinBuf  io.WriteCloser
	stdoutBuf io.Reader
	stderrBuf io.Reader
}

var _ ShellConn = &ShellConnSSH{}

func (c *ShellConnSSH) Stdin() io.Writer {
	return c.stdinBuf
}

func (c *ShellConnSSH) Stdout() io.Reader {
	return c.stdoutBuf
}

func (c *ShellConnSSH) Stderr() io.Reader {
	return c.stderrBuf
}

// Close closes underlying SSH connection.
func (c *ShellConnSSH) Close() {
	c.stdinBuf.Close()
	c.sshSession.Close()
	c.sshClient.Close()
}
