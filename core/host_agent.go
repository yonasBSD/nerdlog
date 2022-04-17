package core

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/juju/errors"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const connectionTimeout = 5 * time.Second

// It's pretty messy, but to keep things optimized for speed as much as we can,
// we have to use different time formats.
//
// queryLogsArgsTimeLayout is used to format the --from and --to arguments for
// nerdlog_query.sh, and it must have leading zeros, since that's how awk's
// strftime formats time and thus that's how the index file has it formatted and
// --from, --to must match exactly. (strftime has %d, which has leading zeros,
// and %e, which has leading spaces, but there are no options to have no
// leading characters at all)
//
// queryLogsMstatsTimeLayout is used to parse the "mstats" (message stats)
// lines output by nerdlog_query.sh, and it uses no leading spaces or zeros,
// because we just operate with awk's "fields" of log lines there, and in
// syslog, those fields are separated by whitespace; so in syslog we get like
// "Apr  9" or "Apr 10", but in mstats we'll have "Apr-9" or "Apr-10".
//
// To avoid reformatting things manually and thus slowing them down, we just use
// different formats.
const queryLogsArgsTimeLayout = "Jan-02-15:04"
const queryLogsMstatsTimeLayout = "Jan-2-15:04"

const syslogTimeLayout = "Jan _2 15:04:05"

type HostAgent struct {
	params HostAgentParams

	connectResCh chan hostConnectRes
	enqueueCmdCh chan hostCmd

	state HostAgentState

	conn *connCtx

	cmdQueue   []hostCmd
	curCmdCtx  *hostCmdCtx
	nextCmdIdx int

	// torndownCh is closed once teardown is completed
	tearingDown bool
	torndownCh  chan struct{}

	//debugFile *os.File
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

	// If TornDown is true, it means it's the last update from that agent.
	TornDown bool
}

type HostAgentUpdateState struct {
	OldState HostAgentState
	NewState HostAgentState
}

type HostAgentParams struct {
	Config ConfigHost

	// ClientID is just an arbitrary string (should be filename-friendly though)
	// which will be appended to the nerdlog_query.sh and its cache filenames.
	//
	// Needed to make sure that different clients won't get conflicts over those
	// files when using the tool concurrently on the same nodes.
	ClientID string

	UpdatesCh chan<- *HostAgentUpdate
}

func NewHostAgent(params HostAgentParams) *HostAgent {
	ha := &HostAgent{
		params: params,

		state:        HostAgentStateDisconnected,
		enqueueCmdCh: make(chan hostCmd, 32),

		torndownCh: make(chan struct{}),
	}

	//debugFile, _ := os.Create("/tmp/host_agent_debug.log")
	//ha.debugFile = debugFile

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
	HostAgentStateDisconnecting HostAgentState = "disconnecting"
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
		// Initiate disconnect
		ha.conn.stdinBuf.Close()
		ha.conn.sshSession.Close()
		ha.conn.sshClient.Close()
	}

	switch oldState {
	case HostAgentStateConnecting:
		ha.connectResCh = nil
	case HostAgentStateConnectedBusy:
		ha.curCmdCtx = nil
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

	case HostAgentStateConnectedIdle:
		if len(ha.cmdQueue) > 0 {
			nextCmd := ha.cmdQueue[0]
			ha.cmdQueue = ha.cmdQueue[1:]

			ha.startCmd(nextCmd)
		}

	case HostAgentStateDisconnected:
		ha.conn = nil
	}
}

func (ha *HostAgent) sendCmdResp(resp interface{}, err error) {
	if ha.curCmdCtx.cmd.respCh == nil {
		return
	}

	ha.curCmdCtx.cmd.respCh <- hostCmdRes{
		hostname: ha.params.Config.Name,
		resp:     resp,
		err:      err,
	}
}

func (ha *HostAgent) run() {
	ticker := time.NewTicker(5 * time.Second)
	var lastUpdTime time.Time

	for {
		select {
		case res := <-ha.connectResCh:
			if res.err != nil {
				fmt.Println("failed to connect:", res.err)
				// TODO: backoff
				ha.changeState(HostAgentStateDisconnected)
				ha.changeState(HostAgentStateConnecting)
				continue
			}

			lastUpdTime = time.Now()

			ha.conn = res.conn
			ha.changeState(HostAgentStateConnectedIdle)

			// Send bootstrap command
			ha.startCmd(hostCmd{
				bootstrap: &hostCmdBootstrap{},
			})

		case cmd := <-ha.enqueueCmdCh:
			if !isStateConnected(ha.state) {
				ha.sendCmdResp(nil, errors.Errorf("not connected"))
				continue
			}

			if ha.state == HostAgentStateConnectedIdle {
				ha.startCmd(cmd)
			} else {
				ha.addCmdToQueue(cmd)
			}

		case line, ok := <-ha.conn.getStdoutLinesCh():
			if !ok {
				// Stdout was just closed
				ha.conn.stdoutLinesCh = nil
				ha.checkIfDisconnected()
				continue
			}

			lastUpdTime = time.Now()

			//if ha.params.Config.Name == "my-host-10" {
			//fmt.Fprintln(ha.debugFile, "rx:", line)
			//}

			switch ha.state {
			case HostAgentStateConnectedBusy:
				switch {
				case ha.curCmdCtx.cmd.bootstrap != nil:
					if line == "bootstrap ok" {
						ha.curCmdCtx.bootstrapCtx.receivedSuccess = true
					} else if line == "bootstrap failed" {
						ha.curCmdCtx.bootstrapCtx.receivedFailure = true
					}

				case ha.curCmdCtx.cmd.ping != nil:
					// Nothing special to do

				case ha.curCmdCtx.cmd.queryLogs != nil:
					// TODO: collect all the info into ha.curCmdCtx.queryLogsCtx
					respCtx := ha.curCmdCtx.queryLogsCtx
					resp := respCtx.Resp

					switch {
					case strings.HasPrefix(line, "mstats:"):
						parts := strings.Split(strings.TrimPrefix(line, "mstats:"), ",")
						if len(parts) < 2 {
							err := errors.Errorf("malformed mstats %q: expected at least 2 parts", line)
							resp.Errs = append(resp.Errs, err)
							continue
						}

						t, err := time.Parse(queryLogsMstatsTimeLayout, parts[0])
						if err != nil {
							resp.Errs = append(resp.Errs, errors.Annotatef(err, "parsing mstats"))
							continue
						}

						t = InferYear(t)

						n, err := strconv.Atoi(parts[1])
						if err != nil {
							resp.Errs = append(resp.Errs, errors.Annotatef(err, "parsing mstats"))
							continue
						}

						resp.MinuteStats[t.Unix()] = MinuteStatsItem{
							NumMsgs: n,
						}

					case strings.HasPrefix(line, "logfile:"):
						msg := strings.TrimPrefix(line, "logfile:")
						idx := strings.IndexRune(msg, ':')
						if idx <= 0 {
							resp.Errs = append(resp.Errs, errors.Errorf("parsing logfile msg: no number of lines %q", line))
							continue
						}

						logFilename := msg[:idx]
						logNumberOfLinesStr := msg[idx+1:]
						logNumberOfLines, err := strconv.Atoi(logNumberOfLinesStr)
						if err != nil {
							resp.Errs = append(resp.Errs, errors.Errorf("parsing logfile msg: invalid number in %q", line))
							continue
						}

						respCtx.logfiles = append(respCtx.logfiles, logfileWithStartingLinenumber{
							filename:       logFilename,
							fromLinenumber: logNumberOfLines,
						})

					case strings.HasPrefix(line, "msg:"):
						// msg:Mar 26 17:08:34 localhost myapp[21134]: Mar 26 17:08:34.476329 foo bar foo bar
						msg := strings.TrimPrefix(line, "msg:")
						idx := strings.IndexRune(msg, ':')
						if idx <= 0 {
							resp.Errs = append(resp.Errs, errors.Errorf("parsing log msg: no line number in %q", line))
							continue
						}

						logLinenoStr := msg[:idx]
						msg = msg[idx+1:]

						logLinenoCombined, err := strconv.Atoi(logLinenoStr)
						if err != nil {
							resp.Errs = append(resp.Errs, errors.Errorf("parsing log msg: invalid line number in %q", line))
							continue
						}

						var logFilename string
						logLineno := logLinenoCombined

						for i := len(respCtx.logfiles) - 1; i >= 0; i-- {
							logfile := respCtx.logfiles[i]
							if logLineno > logfile.fromLinenumber {
								logLineno -= logfile.fromLinenumber
								logFilename = logfile.filename
								break
							}
						}

						origLine := msg

						//if msg == "" {
						//continue
						//}

						// Mar  6 17:08:35 localhost redacted[21
						// Mar 26 17:08:35 localhost redacted[21

						// Find the index of third space, which would indicate where
						// timestamp ends
						ts1Len := 0
						ns := 0
						inWhitespace := false
						for i, r := range msg {
							if r != ' ' {
								inWhitespace = false
								continue
							}

							if inWhitespace {
								continue
							}

							inWhitespace = true

							ns++
							if ns >= 3 {
								ts1Len = i
								break
							}
						}

						if ts1Len == 0 {
							resp.Errs = append(resp.Errs, errors.Errorf("parsing log msg: no time in %q", line))
							continue
						}

						tsStr := msg[:ts1Len]
						msg = msg[ts1Len+1:]
						colonIdx := strings.IndexRune(msg, ':')
						if colonIdx == -1 {
							resp.Errs = append(resp.Errs, errors.Errorf("parsing log msg: no systemd colon"))
							continue
						}

						msg = msg[colonIdx+1:]
						msg = strings.TrimSpace(msg)

						// Logs often contain double timestamps: one from systemd and
						// the next one from the app, so check if the second one exists
						// indeed.
						ts2Idx := strings.Index(msg, tsStr)
						if ts2Idx >= 0 {
							tmp := strings.IndexRune(msg[ts2Idx+ts1Len:], ' ')
							if tmp > 0 {
								ts2Len := ts1Len + tmp
								tsStr = msg[ts2Idx:ts2Len]
								msg = msg[ts2Idx+ts2Len+1:]
							}
						}

						// Now, tsStr contains the timestamp to parse, and msg contains
						// everything after that timestamp.

						t, err := time.Parse(syslogTimeLayout, tsStr)
						if err != nil {
							resp.Errs = append(resp.Errs, errors.Annotatef(err, "parsing log msg"))
							continue
						}

						t = InferYear(t)

						lastTime := respCtx.lastTime
						decreasedTimestamp := false

						if t.Before(lastTime) {
							// Time has decreased: this might happen if the previous log line had
							// a precise timestamp with microseconds, but the current line only has
							// a second precision. Then we just hackishly set the current timestamp
							// to be the same.
							t = lastTime
							decreasedTimestamp = true
						}

						ctxMap := map[string]string{
							"source": ha.params.Config.Name,
						}

						// Extract context tags from msg
						lastEqIdx := strings.LastIndexByte(msg, '=')
					tagsLoop:
						for ; lastEqIdx >= 0; lastEqIdx = strings.LastIndexByte(msg, '=') {
							val := msg[lastEqIdx+1:]
							msg = msg[:lastEqIdx]
							var key string

							lastSpaceIdx := strings.LastIndexByte(msg, ' ')
							if lastSpaceIdx >= 0 {
								key = msg[lastSpaceIdx+1:]
								msg = msg[:lastSpaceIdx]
							} else {
								key = msg
								msg = ""
							}

							for _, r := range key {
								if !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '_' && r != '-' {
									continue tagsLoop
								}
							}

							ctxMap[key] = val
						}

						resp.Logs = append(resp.Logs, LogMsg{
							Time:               t,
							DecreasedTimestamp: decreasedTimestamp,

							LogFilename:   logFilename,
							LogLinenumber: logLineno,

							CombinedLinenumber: logLinenoCombined,

							Msg:     msg,
							Context: ctxMap,

							OrigLine: origLine,
						})

						respCtx.lastTime = t
					}
				}

				if strings.HasPrefix(line, "command_done:") {
					parts := strings.Split(line, ":")
					if len(parts) != 2 {
						fmt.Println("received malformed command_done line:", line)
						continue
					}

					rxIdx, err := strconv.Atoi(parts[1])
					if err != nil {
						fmt.Println("received malformed command_done line:", line)
						continue
					}

					if rxIdx != ha.curCmdCtx.idx {
						fmt.Printf("received unexpected index with command_done: waiting for %d, got %d\n", ha.curCmdCtx.idx, rxIdx)
						continue
					}

					// Current command is done

					switch {
					case ha.curCmdCtx.cmd.bootstrap != nil:
						ha.sendCmdResp(nil, nil)
						if ha.curCmdCtx.bootstrapCtx.receivedSuccess {
							ha.changeState(HostAgentStateConnectedIdle)
						} else {
							// TODO: proper disconnection
							fmt.Println("bootstrap not successful")
							ha.changeState(HostAgentStateDisconnected)
						}

					case ha.curCmdCtx.cmd.ping != nil:
						ha.sendCmdResp(nil, nil)
						ha.changeState(HostAgentStateConnectedIdle)

					case ha.curCmdCtx.cmd.queryLogs != nil:
						resp := ha.curCmdCtx.queryLogsCtx.Resp

						var err error
						if len(resp.Errs) > 0 {
							//if len(resp.Errs) == 1 {
							//err = resp.Errs[0]
							//} else {
							//err = errors.Errorf("%s and %d more errors", resp.Errs[0], len(resp.Errs))
							//}

							ss := []string{}
							for _, e := range resp.Errs {
								ss = append(ss, e.Error())
							}

							err = errors.Errorf("%d errors: %s", len(resp.Errs), strings.Join(ss, "; "))
						}

						ha.sendCmdResp(resp, err)
						ha.changeState(HostAgentStateConnectedIdle)

					default:
						panic(fmt.Sprintf("unhandled cmd %+v", ha.curCmdCtx.cmd))
					}
				}
			}

		case line, ok := <-ha.conn.getStderrLinesCh():
			if !ok {
				// Stderr was just closed
				ha.conn.stderrLinesCh = nil
				ha.checkIfDisconnected()
				continue
			}

			lastUpdTime = time.Now()

			// TODO maybe save somewhere for debugging
			//if ha.params.Config.Name == "my-host-01" {
			//fmt.Println("rxe:", line)
			//}
			_ = line

			//case data := <-ha.stdinCh:
			//ha.stdinBuf.Write([]byte(data))
			//if len(data) > 0 && data[len(data)-1] != '\n' {
			//ha.stdinBuf.Write([]byte("\n"))
			//}

		case <-ticker.C:
			if ha.state == HostAgentStateConnectedIdle && time.Since(lastUpdTime) > 40*time.Second {
				ha.startCmd(hostCmd{
					ping: &hostCmdPing{},
				})
			}

		case <-ha.torndownCh:
			ha.sendUpdate(&HostAgentUpdate{
				TornDown: true,
			})
			return
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

	var sshClient *ssh.Client

	conf := getClientConfig(config.User)
	//fmt.Printf("hey %+v %q\n", config, config.Addr)
	//fmt.Println("hey2", conn, conf)

	if true {
		// Use jumphost
		jumphost, err := getJumphostClient()
		if err != nil {
			res.err = errors.Annotatef(err, "getting jumphost client")
			return res
		}

		conn, err := dialWithTimeout(jumphost, "tcp", config.Addr, connectionTimeout)
		if err != nil {
			res.err = errors.Trace(err)
			return res
		}

		authConn, chans, reqs, err := ssh.NewClientConn(conn, config.Addr, conf)
		if err != nil {
			res.err = errors.Trace(err)
			return res
		}

		sshClient = ssh.NewClient(authConn, chans, reqs)
	} else {
		var err error
		sshClient, err = ssh.Dial("tcp", config.Addr, conf)
		if err != nil {
			res.err = errors.Trace(err)
			return res
		}
	}
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
		//fmt.Println("Connecting to jumphost...")
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

		//fmt.Println("Jumphost ok")
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
		defer func() {
			close(linesCh)
		}()

		scanner := bufio.NewScanner(reader)
		// TODO: also defer signal to reconnect

		for scanner.Scan() {
			linesCh <- scanner.Text()
		}

		if err := scanner.Err(); err != nil {
			//fmt.Println("stdin read error", err)
			return
		} else {
			//fmt.Println("stdin EOF")
		}

		//fmt.Println("stopped reading stdin")
	}
}

func (ha *HostAgent) EnqueueCmd(cmd hostCmd) {
	ha.enqueueCmdCh <- cmd
}

func (ha *HostAgent) Close() {
	ha.EnqueueCmd(hostCmd{
		teardown: true,
	})
}

func (ha *HostAgent) addCmdToQueue(cmd hostCmd) {
	ha.cmdQueue = append(ha.cmdQueue, cmd)
}

func (ha *HostAgent) startCmd(cmd hostCmd) {
	cmdCtx := &hostCmdCtx{
		cmd: cmd,
		idx: ha.nextCmdIdx,
	}

	ha.curCmdCtx = cmdCtx
	ha.nextCmdIdx++

	switch {
	case cmdCtx.cmd.bootstrap != nil:
		cmdCtx.bootstrapCtx = &hostCmdCtxBootstrap{}

		ha.conn.stdinBuf.Write([]byte("cat <<- 'EOF' > /var/tmp/nerdlog_query_" + ha.params.ClientID + ".sh\n" + nerdlogQuerySh + "EOF\n"))
		ha.conn.stdinBuf.Write([]byte("if [ $? == 0 ]; then echo 'bootstrap ok'; else echo 'bootstrap failed'; fi\n"))

	case cmdCtx.cmd.ping != nil:
		cmdCtx.pingCtx = &hostCmdCtxPing{}

		ha.conn.stdinBuf.Write([]byte("whoami\n"))

	case cmdCtx.cmd.queryLogs != nil:
		cmdCtx.queryLogsCtx = &hostCmdCtxQueryLogs{
			Resp: &LogResp{
				MinuteStats: map[int64]MinuteStatsItem{},
			},
		}

		parts := []string{
			"bash /var/tmp/nerdlog_query_" + ha.params.ClientID + ".sh",
			"--cache-file", "/tmp/nerdlog_query_cache_" + ha.params.ClientID,
		}

		if !cmdCtx.cmd.queryLogs.from.IsZero() {
			parts = append(parts, "--from", cmdCtx.cmd.queryLogs.from.UTC().Format(queryLogsArgsTimeLayout))
		}

		if !cmdCtx.cmd.queryLogs.to.IsZero() {
			parts = append(parts, "--to", cmdCtx.cmd.queryLogs.to.UTC().Format(queryLogsArgsTimeLayout))
		}

		if cmdCtx.cmd.queryLogs.linesUntil > 0 {
			parts = append(parts, "--lines-until", strconv.Itoa(cmdCtx.cmd.queryLogs.linesUntil))
		}

		if cmdCtx.cmd.queryLogs.query != "" {
			parts = append(parts, "'"+cmdCtx.cmd.queryLogs.query+"'")
		}

		cmd := strings.Join(parts, " ") + "\n"
		//fmt.Println("hey", ha.params.Config.Name, "cmd:", cmd)

		//if ha.params.Config.Name == "my-host-10" {
		//fmt.Fprintln(ha.debugFile, "cmd:", ha.params.Config.Name, ":", cmd)
		//}

		ha.conn.stdinBuf.Write([]byte(cmd))

	case cmdCtx.cmd.teardown:
		ha.tearingDown = true
		ha.changeState(HostAgentStateDisconnecting)

	default:
		panic(fmt.Sprintf("invalid command %+v", cmdCtx.cmd))
	}

	ha.conn.stdinBuf.Write([]byte(fmt.Sprintf("echo 'command_done:%d'\n", cmdCtx.idx)))

	ha.changeState(HostAgentStateConnectedBusy)
}

func (ha *HostAgent) checkIfDisconnected() {
	if ha.conn.stderrLinesCh == nil && ha.conn.stdoutLinesCh == nil {
		// We're fully disconnected
		ha.changeState(HostAgentStateDisconnected)

		if ha.tearingDown {
			close(ha.torndownCh)
		} else {
			ha.changeState(HostAgentStateConnecting)
		}
	}
}

func logError(err error) {
	// TODO: log them to some file instead
	fmt.Println("ERROR:", err.Error())
}

// InferYear infers year from the month of the given timestamp, and the current
// time. Resulting timestamp (with the year populated) is then returned.
//
// Most of the time it just uses the current year, but on the year boundary
// it can return previous or next year.
func InferYear(t time.Time) time.Time {
	now := time.Now()

	// If month of the syslog being parsed is the same as the current month, just
	// use the current year.
	if now.Month() == t.Month() {
		return timeWithYear(t, now.Year())
	}

	// Month of the syslog is different from the current month, so we need to
	// have logic for the boundary of the year.

	if t.Month() == time.December && now.Month() == time.January {
		// We're in January now and we're parsing some logs from December.
		return timeWithYear(t, now.Year()-1)
	} else if t.Month() == time.January && now.Month() == time.December {
		// We're in December now and we're parsing some logs from January.
		// It's weird to get timestamp from the future, but better to have a case
		// for that.
		return timeWithYear(t, now.Year()+1)
	}

	// For all other cases, still use the current year.
	return timeWithYear(t, now.Year())
}

func timeWithYear(t time.Time, year int) time.Time {
	return time.Date(
		year,
		t.Month(),
		t.Day(),

		t.Hour(),
		t.Minute(),
		t.Second(),
		t.Nanosecond(),
		t.Location(),
	)
}
