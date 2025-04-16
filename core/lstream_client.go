package core

import (
	"bufio"
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
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

	"github.com/dimonomid/nerdlog/log"
)

const connectionTimeout = 5 * time.Second

// Setting useGzip to false is just a simple way to disable gzip, for debugging
// purposes or w/e, since it's still experimental. Maybe we need to add a flag
// for it, we'll see.
const useGzip = true

const (
	// gzipStartMarker and gzipEndMarker are echoed in the beginning and the end
	// of the gzipped output. Effectively we're doing this:
	//
	//   $ echo gzip_start ; whatever command we need to run | gzip ; echo gzip_end
	//
	// and the scanner func (returned by getScannerFunc) sees those markers and
	// buffers gzipped output until it's done, then gunzips it and sends to the
	// clients, so it's totally opaque for them.
	gzipStartMarker = "gzip_start"
	gzipEndMarker   = "gzip_end"
)

// It's pretty messy, but to keep things optimized for speed as much as we can,
// we have to use different time formats.
//
// queryLogsArgsTimeLayout is used to format the --from and --to arguments for
// nerdlog_agent.sh, and it must have leading zeros, since that's how awk's
// strftime formats time and thus that's how the index file has it formatted and
// --from, --to must match exactly. (strftime has %d, which has leading zeros,
// and %e, which has leading spaces, but there are no options to have no
// leading characters at all)
//
// queryLogsMstatsTimeLayout is used to parse the "mstats" (message stats)
// lines output by nerdlog_agent.sh, and it uses no leading spaces or zeros,
// because we just operate with awk's "fields" of log lines there, and in
// syslog, those fields are separated by whitespace; so in syslog we get like
// "Apr  9" or "Apr 10", but in mstats we'll have "Apr-9" or "Apr-10".
//
// To avoid reformatting things manually and thus slowing them down, we just use
// different formats.
const queryLogsArgsTimeLayout = "2006-01-02-15:04"
const queryLogsMstatsTimeLayout = "Jan2-15:04"

const syslogTimeLayout = "Jan _2 15:04:05"

//go:embed nerdlog_agent.sh
var nerdlogAgentSh string

type LStreamClient struct {
	params LStreamClientParams

	connectResCh chan lstreamConnRes
	enqueueCmdCh chan lstreamCmd

	// timezone is a string received from the logstream
	timezone string
	// location is loaded based on the timezone. If failed, it'll be UTC.
	location *time.Location

	numConnAttempts int

	state     LStreamClientState
	busyStage BusyStage

	conn *connCtx

	cmdQueue   []lstreamCmd
	curCmdCtx  *lstreamCmdCtx
	nextCmdIdx int

	// disconnectReqCh is sent to when Close is called.
	disconnectReqCh chan disconnectReq
	tearingDown     bool
	// disconnectedBeforeTeardownCh is closed once tearingDown is true and we're
	// fully disconnected.
	disconnectedBeforeTeardownCh chan struct{}

	//debugFile *os.File
}

// disconnectReq represents a request to abort whatever connection we have,
// and either teardown or reconnect again.
type disconnectReq struct {
	// If teardown is true, it means the LStreamClient should completely stop. Otherwise,
	// after disconnecting, it will reconnect.
	teardown bool

	// If changeName is non-empty, the LStreamClient's Name will be updated;
	// it's useful for teardowns, to distinguish from potentially-existing
	// another LStreamClient with the same (old) name.
	changeName string
}

type connCtx struct {
	sshClient  *ssh.Client
	sshSession *ssh.Session
	stdinBuf   io.WriteCloser

	stdoutLinesCh chan string
	stderrLinesCh chan string
}

type BusyStage struct {
	// Num is just a stage number. Its meaning depends on the kind of command the
	// host is executing, but a general rule is that this number starts from 1
	// and then increases, as the process goes on, so we can compare different
	// nodes and e.g. find the slowest one.
	Num int

	// Title is just a human-readable description of the stage.
	Title string

	// Percentage is a percentage of the current stage.
	Percentage int
}

type ConnDetails struct {
	// Err is an error message from the last connection attempt.
	Err string
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

// LStreamClientUpdate represents an update from logstream client. Name is always
// populated and it's the logstream's name, and from all the other fields, exactly
// one field must be non-nil.
type LStreamClientUpdate struct {
	Name string

	State *LStreamClientUpdateState

	ConnDetails *ConnDetails
	BusyStage   *BusyStage

	// If TornDown is true, it means it's the last update from that client.
	TornDown bool
}

type LStreamClientUpdateState struct {
	OldState LStreamClientState
	NewState LStreamClientState
}

type LStreamClientParams struct {
	LogStream LogStream

	Logger *log.Logger

	// ClientID is just an arbitrary string (should be filename-friendly though)
	// which will be appended to the nerdlog_agent.sh and its cache filenames.
	//
	// Needed to make sure that different clients won't get conflicts over those
	// files when using the tool concurrently on the same nodes.
	ClientID string

	UpdatesCh chan<- *LStreamClientUpdate
}

func NewLStreamClient(params LStreamClientParams) *LStreamClient {
	params.Logger = params.Logger.WithNamespaceAppended(
		fmt.Sprintf("LSClient_%s", params.LogStream.Name),
	)

	lsc := &LStreamClient{
		params: params,

		timezone: "UTC",
		location: time.UTC,

		state:        LStreamClientStateDisconnected,
		enqueueCmdCh: make(chan lstreamCmd, 32),

		disconnectReqCh:              make(chan disconnectReq, 1),
		disconnectedBeforeTeardownCh: make(chan struct{}),
	}

	//debugFile, _ := os.Create("/tmp/lsclient_debug.log")
	//lsc.debugFile = debugFile

	lsc.changeState(LStreamClientStateConnecting)

	go lsc.run()

	return lsc
}

func (lsc *LStreamClient) SendFoo() {
}

type LStreamClientState string

const (
	LStreamClientStateDisconnected  LStreamClientState = "disconnected"
	LStreamClientStateConnecting    LStreamClientState = "connecting"
	LStreamClientStateDisconnecting LStreamClientState = "disconnecting"
	LStreamClientStateConnectedIdle LStreamClientState = "connected_idle"
	LStreamClientStateConnectedBusy LStreamClientState = "connected_busy"
)

func isStateConnected(state LStreamClientState) bool {
	return state == LStreamClientStateConnectedIdle || state == LStreamClientStateConnectedBusy
}

func (lsc *LStreamClient) changeState(newState LStreamClientState) {
	oldState := lsc.state

	// Properly leave old state

	if isStateConnected(oldState) && !isStateConnected(newState) {
		// Initiate disconnect
		lsc.conn.stdinBuf.Close()
		lsc.conn.sshSession.Close()
		lsc.conn.sshClient.Close()
	}

	switch oldState {
	case LStreamClientStateConnecting:
		lsc.connectResCh = nil
	case LStreamClientStateConnectedBusy:
		lsc.curCmdCtx = nil
		lsc.busyStage = BusyStage{}
	}

	// Enter new state

	lsc.state = newState
	lsc.sendUpdate(&LStreamClientUpdate{
		State: &LStreamClientUpdateState{
			OldState: oldState,
			NewState: newState,
		},
	})

	switch lsc.state {
	case LStreamClientStateConnecting:
		lsc.numConnAttempts++
		lsc.connectResCh = make(chan lstreamConnRes, 1)
		go connectToLogStream(lsc.params.Logger, lsc.params.LogStream, lsc.connectResCh)

	case LStreamClientStateConnectedIdle:
		if len(lsc.cmdQueue) > 0 {
			nextCmd := lsc.cmdQueue[0]
			lsc.cmdQueue = lsc.cmdQueue[1:]

			lsc.startCmd(nextCmd)
		}

	case LStreamClientStateDisconnected:
		lsc.conn = nil
	}
}

func (lsc *LStreamClient) sendBusyStageUpdate() {
	upd := lsc.busyStage
	lsc.sendUpdate(&LStreamClientUpdate{
		BusyStage: &upd,
	})
}

func (lsc *LStreamClient) sendCmdResp(resp interface{}, err error) {
	if lsc.curCmdCtx == nil {
		return
	}

	if lsc.curCmdCtx.cmd.respCh == nil {
		return
	}

	lsc.curCmdCtx.cmd.respCh <- lstreamCmdRes{
		hostname: lsc.params.LogStream.Name,
		resp:     resp,
		err:      err,
	}
}

func (lsc *LStreamClient) run() {
	ticker := time.NewTicker(1 * time.Second)
	var connectAfter time.Time
	var lastUpdTime time.Time

	for {
		select {
		case res := <-lsc.connectResCh:
			if res.err != nil {
				lsc.sendUpdate(&LStreamClientUpdate{
					ConnDetails: &ConnDetails{
						Err: fmt.Sprintf("attempt %d: %s", lsc.numConnAttempts, res.err.Error()),
					},
				})

				lsc.changeState(LStreamClientStateDisconnected)
				if lsc.tearingDown {
					close(lsc.disconnectedBeforeTeardownCh)
					continue
				}

				connectAfter = time.Now().Add(2 * time.Second)
				continue
			}

			lsc.numConnAttempts = 0

			lastUpdTime = time.Now()

			lsc.conn = res.conn
			lsc.changeState(LStreamClientStateConnectedIdle)

			// Send bootstrap command
			lsc.startCmd(lstreamCmd{
				bootstrap: &lstreamCmdBootstrap{},
			})

		case cmd := <-lsc.enqueueCmdCh:
			// Require a connection.
			if !isStateConnected(lsc.state) {
				lsc.sendCmdResp(nil, errors.Errorf("not connected"))
				continue
			}

			// And then, depending on whether we're busy or idle, either act
			// right away, or enqueue for later.
			if lsc.state == LStreamClientStateConnectedIdle {
				lsc.startCmd(cmd)
			} else {
				lsc.addCmdToQueue(cmd)
			}

		case line, ok := <-lsc.conn.getStdoutLinesCh():
			if !ok {
				// Stdout was just closed
				lsc.conn.stdoutLinesCh = nil
				lsc.checkIfDisconnected()
				continue
			}

			//lsc.params.Logger.Verbose1f("hey line(%s): %s", lsc.params.LogStream.Name, line)

			lastUpdTime = time.Now()

			switch lsc.state {
			case LStreamClientStateConnectedBusy:
				if lsc.curCmdCtx == nil {
					// We received some line before printing any command, must be
					// just standard welcome message, but we're not interested in that.
					continue
				}

				switch {
				case lsc.curCmdCtx.cmd.bootstrap != nil:
					tzPrefix := "host_timezone:"
					if strings.HasPrefix(line, tzPrefix) {
						tz := strings.TrimPrefix(line, tzPrefix)
						lsc.params.Logger.Verbose1f("Got logstream timezone: %s\n", tz)

						location, err := time.LoadLocation(tz)
						if err != nil {
							lsc.params.Logger.Errorf("Error: failed to load location %s, will use UTC\n", tz)
							// TODO: send an update and then the receiver should show a message
							// to the user
						} else {
							lsc.timezone = tz
							lsc.location = location
						}
					}

					if line == "bootstrap ok" {
						lsc.curCmdCtx.bootstrapCtx.receivedSuccess = true
					} else if line == "bootstrap failed" {
						lsc.curCmdCtx.bootstrapCtx.receivedFailure = true
					}

				case lsc.curCmdCtx.cmd.ping != nil:
					// Nothing special to do

				case lsc.curCmdCtx.cmd.queryLogs != nil:
					// TODO: collect all the info into lsc.curCmdCtx.queryLogsCtx
					respCtx := lsc.curCmdCtx.queryLogsCtx
					resp := respCtx.Resp

					switch {
					case strings.HasPrefix(line, "s:"):
						parts := strings.Split(strings.TrimPrefix(line, "s:"), ",")
						if len(parts) < 2 {
							err := errors.Errorf("malformed mstats %q: expected at least 2 parts", line)
							resp.Errs = append(resp.Errs, err)
							continue
						}

						t, err := time.ParseInLocation(queryLogsMstatsTimeLayout, parts[0], lsc.location)
						if err != nil {
							resp.Errs = append(resp.Errs, errors.Annotatef(err, "parsing mstats"))
							continue
						}

						t = InferYear(t)
						t = t.UTC()

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
							resp.Errs = append(resp.Errs, errors.Annotatef(err, "parsing logfile msg: invalid number in %q", line))
							continue
						}

						respCtx.logfiles = append(respCtx.logfiles, logfileWithStartingLinenumber{
							filename:       logFilename,
							fromLinenumber: logNumberOfLines,
						})

					case strings.HasPrefix(line, "m:"):
						// msg:Mar 26 17:08:34 localhost myapp[21134]: Mar 26 17:08:34.476329 foo bar foo bar
						msg := strings.TrimPrefix(line, "m:")
						idx := strings.IndexRune(msg, ':')
						if idx <= 0 {
							resp.Errs = append(resp.Errs, errors.Errorf("parsing log msg: no line number in %q", line))
							continue
						}

						logLinenoStr := msg[:idx]
						msg = msg[idx+1:]

						logLinenoCombined, err := strconv.Atoi(logLinenoStr)
						if err != nil {
							resp.Errs = append(resp.Errs, errors.Annotatef(err, "parsing log msg: invalid line number in %q", line))
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

						parseRes, err := parseLine(lsc.location, msg, respCtx.lastTime)
						if err != nil {
							resp.Errs = append(resp.Errs, errors.Annotatef(err, "parsing log msg: no time in %q", line))
							continue
						}

						parseRes.ctxMap["source"] = lsc.params.LogStream.Name

						resp.Logs = append(resp.Logs, LogMsg{
							Time:               parseRes.time,
							DecreasedTimestamp: parseRes.decreasedTimestamp,

							LogFilename:   logFilename,
							LogLinenumber: logLineno,

							CombinedLinenumber: logLinenoCombined,

							Msg:     parseRes.msg,
							Context: parseRes.ctxMap,

							OrigLine: origLine,
						})

						respCtx.lastTime = parseRes.time

						// NOTE: the "p:" lines (process-related) are in stderr and thus
						// are handled below. Why they are in stderr, see comments there.
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

					if rxIdx != lsc.curCmdCtx.idx {
						fmt.Printf("received unexpected index with command_done: waiting for %d, got %d\n", lsc.curCmdCtx.idx, rxIdx)
						continue
					}

					// Current command is done.

					// NOTE: it's tricky to get the exit code from the command we just
					// ran, because when querying logs, we're piping it to gzip, and
					// there's no good shell-independent way to get status code from the
					// first command in the pipe (as opposed to the last one).
					//
					// So here we don't even try. Instead, we have a command-dependent
					// logic to check for errors; e.g. for querying logs, we check stderr
					// for lines starting from "error:", and appending these to resp.Errs.
					// And here (right below), we'll check all these errors that we
					// accumulated.

					switch {
					case lsc.curCmdCtx.cmd.bootstrap != nil:
						lsc.sendCmdResp(nil, nil)
						if lsc.curCmdCtx.bootstrapCtx.receivedSuccess {
							lsc.changeState(LStreamClientStateConnectedIdle)
						} else {
							// TODO: proper disconnection
							fmt.Println("bootstrap not successful")
							lsc.changeState(LStreamClientStateDisconnected)
						}

					case lsc.curCmdCtx.cmd.ping != nil:
						lsc.sendCmdResp(nil, nil)
						lsc.changeState(LStreamClientStateConnectedIdle)

					case lsc.curCmdCtx.cmd.queryLogs != nil:
						resp := lsc.curCmdCtx.queryLogsCtx.Resp

						var err error
						if len(resp.Errs) == 1 {
							err = resp.Errs[0]
						} else if len(resp.Errs) > 0 {
							ss := []string{}
							for _, e := range resp.Errs {
								ss = append(ss, e.Error())
							}

							err = errors.Errorf("%d errors: %s", len(resp.Errs), strings.Join(ss, "; "))
						}

						lsc.sendCmdResp(resp, err)
						lsc.changeState(LStreamClientStateConnectedIdle)

					default:
						panic(fmt.Sprintf("unhandled cmd %+v", lsc.curCmdCtx.cmd))
					}
				}
			}

		case line, ok := <-lsc.conn.getStderrLinesCh():
			if !ok {
				// Stderr was just closed
				lsc.conn.stderrLinesCh = nil
				lsc.checkIfDisconnected()
				continue
			}

			lsc.params.Logger.Verbose2f("hey stderr line(%s): %s", lsc.params.LogStream.Name, line)

			lastUpdTime = time.Now()

			// NOTE: the "p:" lines (process-related) are here in stderr, because
			// stdout is gzipped and thus we don't have any partial results (we get
			// them all at once), but for the process info, we actually want it right
			// when it's printed by the nerdlog_agent.sh.
			switch lsc.state {
			case LStreamClientStateConnectedBusy:

				switch {
				case lsc.curCmdCtx.cmd.queryLogs != nil:
					respCtx := lsc.curCmdCtx.queryLogsCtx
					resp := respCtx.Resp

					switch {
					case strings.HasPrefix(line, "p:"):
						// "p:" means process
						processLine := strings.TrimPrefix(line, "p:")

						switch {
						case strings.HasPrefix(processLine, "stage:"):
							stageLine := strings.TrimPrefix(processLine, "stage:")
							parts := strings.Split(stageLine, ":")
							if len(parts) < 2 {
								fmt.Println("received malformed p:stage line:", line)
								continue
							}

							num, err := strconv.Atoi(parts[0])
							if err != nil {
								fmt.Println("received malformed p:stage line:", line)
								continue
							}

							lsc.busyStage = BusyStage{
								Num:   num,
								Title: parts[1],
							}
							lsc.sendBusyStageUpdate()

						case strings.HasPrefix(processLine, "p:"):
							// second "p:" means percentage

							percentage, err := strconv.Atoi(strings.TrimPrefix(processLine, "p:"))
							if err != nil {
								fmt.Println("received malformed p:p line:", line)
								continue
							}

							lsc.busyStage.Percentage = percentage
							lsc.sendBusyStageUpdate()
						}

					case strings.HasPrefix(line, "error:"):
						// The agent script printed an error; it means that the whole
						// execution will be considered failed once it's done. For now we
						// just add the error to the resulting response.
						errMsg := strings.TrimPrefix(line, "error:")
						resp.Errs = append(resp.Errs, errors.New(errMsg))
					}
				}
			}

			//case data := <-lsc.stdinCh:
			//lsc.stdinBuf.Write([]byte(data))
			//if len(data) > 0 && data[len(data)-1] != '\n' {
			//lsc.stdinBuf.Write([]byte("\n"))
			//}

		case <-ticker.C:
			if lsc.state == LStreamClientStateConnectedIdle && time.Since(lastUpdTime) > 40*time.Second {
				lsc.startCmd(lstreamCmd{
					ping: &lstreamCmdPing{},
				})
			} else if !connectAfter.IsZero() {
				connectAfter = time.Time{}
				lsc.changeState(LStreamClientStateConnecting)
			}

		case req := <-lsc.disconnectReqCh:
			lsc.params.Logger.Infof("Received disconnect message (teardown:%v)", req.teardown)

			if req.teardown {
				lsc.tearingDown = true
			}

			if req.changeName != "" {
				lsc.params.LogStream.Name = req.changeName
			}

			// If we're already disconnected, consider ourselves torn-down already.
			// Otherwise, initiate disconnection.
			if lsc.state == LStreamClientStateDisconnected {
				if req.teardown {
					close(lsc.disconnectedBeforeTeardownCh)
				}
			} else {
				lsc.changeState(LStreamClientStateDisconnecting)
			}

		case <-lsc.disconnectedBeforeTeardownCh:
			lsc.params.Logger.Infof("Teardown completed")
			lsc.sendUpdate(&LStreamClientUpdate{
				TornDown: true,
			})
			return
		}
	}
}

func (lsc *LStreamClient) sendUpdate(upd *LStreamClientUpdate) {
	upd.Name = lsc.params.LogStream.Name
	lsc.params.UpdatesCh <- upd
}

type lstreamConnRes struct {
	conn *connCtx
	err  error
}

func connectToLogStream(
	logger *log.Logger,
	logStream LogStream,
	resCh chan<- lstreamConnRes,
) (res lstreamConnRes) {
	defer func() {
		if res.err != nil {
			logger.Errorf("Connection failed: %s", res.err)
		}

		resCh <- res
	}()

	var sshClient *ssh.Client

	conf := getClientConfig(logger, logStream.Host.User)
	//fmt.Println("hey2", conn, conf)

	if logStream.Jumphost != nil {
		logger.Infof("Connecting via jumphost")
		// Use jumphost
		jumphost, err := getJumphostClient(logger, logStream.Jumphost)
		if err != nil {
			logger.Errorf("Jumphost connection failed: %s", err)
			res.err = errors.Annotatef(err, "getting jumphost client")
			return res
		}

		conn, err := dialWithTimeout(jumphost, "tcp", logStream.Host.Addr, connectionTimeout)
		if err != nil {
			res.err = errors.Trace(err)
			return res
		}

		authConn, chans, reqs, err := ssh.NewClientConn(conn, logStream.Host.Addr, conf)
		if err != nil {
			res.err = errors.Trace(err)
			return res
		}

		sshClient = ssh.NewClient(authConn, chans, reqs)
	} else {
		logger.Infof("Connecting to %s (%+v)", logStream.Host.Addr, conf)
		var err error
		sshClient, err = ssh.Dial("tcp", logStream.Host.Addr, conf)
		if err != nil {
			res.err = errors.Trace(err)
			return res
		}
	}
	//defer client.Close()

	logger.Infof("Connected to %s", logStream.Host.Addr)

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

// scanLinesPreserveCarriageReturn is the same as bufio.ScanLines, but it does
// not strip the \r characters: it's just a hack to support gzipping. In fact,
// since we sometimes read text lines and sometimes gzipped data, we'd better
// use some other custom scanner, but for now just this simple hack.
func scanLinesPreserveCarriageReturn(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		// We have a full newline-terminated line.
		return i + 1, data[0:i], nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}

func getScannerFunc(name string, reader io.Reader, linesCh chan<- string) func() {
	return func() {
		defer func() {
			close(linesCh)
		}()

		scanner := bufio.NewScanner(reader)

		// See comments for scanLinesPreserveCarriageReturn for details why we need
		// this custom split function.
		scanner.Split(scanLinesPreserveCarriageReturn)

		// TODO: also defer signal to reconnect

		// inGzip is true when we're receiving gzipped data.
		// gzipBuf accumulates that data, and once we receive the gzipEndMarker,
		// we gunzip all this data and feed the lines to the channel.
		//
		// TODO: instead of accumulating it and then unpacking all at once, do it
		// gradually as we receive data. Idk how much of an improvement it'd be in
		// practice though, since we're not receiving some huge chunks of data,
		// just a bit nicer.
		inGzip := false
		var gzipBuf bytes.Buffer

		for scanner.Scan() {
			lineBytes := scanner.Bytes()
			line := string(lineBytes)

			if !inGzip && line == gzipStartMarker {
				// Gzipped data begins
				inGzip = true
				gzipBuf.Reset()

				// We also need to continue loop iteration now so that we don't
				// add this gzipStartMarker line to the gzipBuf below.
				continue
			} else if inGzip && strings.HasSuffix(line, gzipEndMarker) {
				// We just reached the end of the gzipped data
				inGzip = false

				// Append this last piece
				gzipBuf.Write(lineBytes[:len(lineBytes)-len(gzipEndMarker)])

				var err error

				// Gunzip the data and feed all the lines to linesCh
				var r io.Reader
				r, err = gzip.NewReader(&gzipBuf)
				if err != nil {
					panic(err.Error())
				}

				scanner := bufio.NewScanner(r)
				for scanner.Scan() {
					linesCh <- scanner.Text()
				}

				continue
			}

			if !inGzip {
				// We're not in gzipped data, so just feed this line directly.
				linesCh <- line
			} else {
				// We're reading gzipped data now, so for now just add it to the
				// gzipBuf (together with the \n which was stripped by the scanner).
				gzipBuf.Write(lineBytes)
				gzipBuf.WriteByte('\n')
			}
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

func (lsc *LStreamClient) EnqueueCmd(cmd lstreamCmd) {
	lsc.enqueueCmdCh <- cmd
}

// Close initiates the shutdown. It doesn't wait for the shutdown to complete;
// client code needs to wait for the corresponding event (with TornDown: true).
//
// If changeName is non-empty, the LStreamClient's Name will be updated; it's
// useful to distinguish this LStreamClient from potentially-existing another one
// with the same (old) name.
func (lsc *LStreamClient) Close(changeName string) {
	select {
	case lsc.disconnectReqCh <- disconnectReq{
		teardown:   true,
		changeName: changeName,
	}:
	default:
	}
}

func (lsc *LStreamClient) Reconnect() {
	select {
	case lsc.disconnectReqCh <- disconnectReq{
		teardown: false,
	}:
	default:
	}
}

func (lsc *LStreamClient) addCmdToQueue(cmd lstreamCmd) {
	lsc.cmdQueue = append(lsc.cmdQueue, cmd)
}

func (lsc *LStreamClient) startCmd(cmd lstreamCmd) {
	cmdCtx := &lstreamCmdCtx{
		cmd: cmd,
		idx: lsc.nextCmdIdx,
	}

	lsc.curCmdCtx = cmdCtx
	lsc.nextCmdIdx++

	switch {
	case cmdCtx.cmd.bootstrap != nil:
		cmdCtx.bootstrapCtx = &lstreamCmdCtxBootstrap{}

		lsc.conn.stdinBuf.Write([]byte("cat <<- 'EOF' > " + lsc.getLStreamNerdlogAgentPath() + "\n" + nerdlogAgentSh + "EOF\n"))
		lsc.conn.stdinBuf.Write([]byte(`echo "host_timezone:$(timedatectl show --property=Timezone --value)"` + "\n"))
		lsc.conn.stdinBuf.Write([]byte("if [[ $? == 0 ]]; then echo 'bootstrap ok'; else echo 'bootstrap failed'; fi\n"))

	case cmdCtx.cmd.ping != nil:
		cmdCtx.pingCtx = &lstreamCmdCtxPing{}

		cmd := "whoami\n"
		lsc.conn.stdinBuf.Write([]byte(cmd))

	case cmdCtx.cmd.queryLogs != nil:
		cmdCtx.queryLogsCtx = &lstreamCmdCtxQueryLogs{
			Resp: &LogResp{
				MinuteStats: map[int64]MinuteStatsItem{},
			},
		}

		var parts []string

		if useGzip {
			parts = append(parts, "echo", gzipStartMarker, ";")
		}

		parts = append(
			parts,
			"bash", lsc.getLStreamNerdlogAgentPath(),
			"query",
			"--cache-file", lsc.getLStreamIndexFilePath(),
			"--max-num-lines", strconv.Itoa(cmdCtx.cmd.queryLogs.maxNumLines),
			"--logfile-last", lsc.params.LogStream.LogFileLast(),
		)

		if logFilePrev, ok := lsc.params.LogStream.LogFilePrev(); ok {
			parts = append(parts, "--logfile-prev", logFilePrev)
		}

		if !cmdCtx.cmd.queryLogs.from.IsZero() {
			parts = append(parts, "--from", cmdCtx.cmd.queryLogs.from.In(lsc.location).Format(queryLogsArgsTimeLayout))
		}

		if !cmdCtx.cmd.queryLogs.to.IsZero() {
			parts = append(parts, "--to", cmdCtx.cmd.queryLogs.to.In(lsc.location).Format(queryLogsArgsTimeLayout))
		}

		if cmdCtx.cmd.queryLogs.linesUntil > 0 {
			parts = append(parts, "--lines-until", strconv.Itoa(cmdCtx.cmd.queryLogs.linesUntil))
		}

		if cmdCtx.cmd.queryLogs.query != "" {
			parts = append(parts, "'"+strings.Replace(cmdCtx.cmd.queryLogs.query, "'", "'\"'\"'", -1)+"'")
		}

		if useGzip {
			parts = append(parts, "|", "gzip", ";", "echo", gzipEndMarker)
		}

		cmd := strings.Join(parts, " ") + "\n"
		lsc.params.Logger.Verbose2f("hey command(%s): %s", lsc.params.LogStream.Name, cmd)

		lsc.conn.stdinBuf.Write([]byte(cmd))

	default:
		panic(fmt.Sprintf("invalid command %+v", cmdCtx.cmd))
	}

	lsc.conn.stdinBuf.Write([]byte(fmt.Sprintf("echo 'command_done:%d'\n", cmdCtx.idx)))

	lsc.changeState(LStreamClientStateConnectedBusy)
}

// getLStreamNerdlogAgentPath returns the logstream-side path to the nerdlog_agent.sh
// for the particular log stream.
func (lsc *LStreamClient) getLStreamNerdlogAgentPath() string {
	return fmt.Sprintf(
		"/tmp/nerdlog_agent_%s_%s.sh",
		lsc.params.ClientID,
		filepathToId(lsc.params.LogStream.LogFileLast()),
	)
}

// getLStreamIndexFilePath returns the logstream-side path to the index file for
// the particular log stream.
func (lsc *LStreamClient) getLStreamIndexFilePath() string {
	return fmt.Sprintf(
		"/tmp/nerdlog_agent_index_%s_%s",
		lsc.params.ClientID,
		filepathToId(lsc.params.LogStream.LogFileLast()),
	)
}

// filepathToId takes a path and returns a string suitable to be used as
// part of a filename (with all slashes removed).
func filepathToId(p string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_")
	return replacer.Replace(p)
}

func (lsc *LStreamClient) checkIfDisconnected() {
	if lsc.conn.stderrLinesCh == nil && lsc.conn.stdoutLinesCh == nil {
		// We're fully disconnected
		lsc.changeState(LStreamClientStateDisconnected)

		if lsc.tearingDown {
			close(lsc.disconnectedBeforeTeardownCh)
		} else {
			lsc.changeState(LStreamClientStateConnecting)
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

type parseSystemdMsgResult struct {
	tsStr  string
	ts1Len int

	msg string
}

func parseSystemdMsg(msg string) (*parseSystemdMsgResult, error) {
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
		return nil, errors.Errorf("parsing log msg: no time in %q", msg)
	}

	tsStr := msg[:ts1Len]
	msg = msg[ts1Len+1:]
	colonIdx := strings.IndexRune(msg, ':')
	if colonIdx == -1 {
		return nil, errors.Errorf("parsing log msg: no systemd colon")
	}

	msg = msg[colonIdx+1:]
	msg = strings.TrimSpace(msg)

	return &parseSystemdMsgResult{
		tsStr:  tsStr,
		ts1Len: ts1Len,
		msg:    msg,
	}, nil
}

type parseLineResult struct {
	// time might or might not be populated: most of our messages contain an
	// extra (more precise) timestamp, so in this case it'll be populated here,
	// and client code should use it.
	time               time.Time
	decreasedTimestamp bool

	// msg is the actual log message.
	msg string

	// ctxMap contains context for the message.
	ctxMap map[string]string
}

func parseLine(loc *time.Location, msg string, lastTime time.Time) (*parseLineResult, error) {
	sdmsg, err := parseSystemdMsg(msg)
	if err != nil {
		return nil, errors.Trace(err)
	}

	msg = sdmsg.msg

	if len(msg) > 0 && msg[0] == '{' {
		res, err := parseLineStructured(loc, lastTime, sdmsg)
		if err == nil {
			return res, nil
		}

		// The message looked like it was in a structured format, but we failed
		// to parse it, so fallback to the regular parsing
	}

	res, err := parseLineUnstructured(loc, lastTime, sdmsg)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return res, nil
}

func parseLineUnstructured(
	loc *time.Location, lastTime time.Time, sdmsg *parseSystemdMsgResult,
) (*parseLineResult, error) {
	tsStr := sdmsg.tsStr
	msg := sdmsg.msg

	// Logs often contain double timestamps: one from systemd and
	// the next one from the app, so check if the second one exists
	// indeed.
	ts2Idx := strings.Index(msg, tsStr)
	if ts2Idx >= 0 {
		tmp := strings.IndexRune(msg[ts2Idx+sdmsg.ts1Len:], ' ')
		if tmp > 0 {
			ts2Len := sdmsg.ts1Len + tmp
			tsStr = msg[ts2Idx : ts2Idx+ts2Len]
			msg = msg[ts2Idx+ts2Len+1:]
		}
	}

	// Now, tsStr contains the timestamp to parse, and msg contains
	// everything after that timestamp.

	t, err := time.ParseInLocation(syslogTimeLayout, tsStr, loc)
	if err != nil {
		return nil, errors.Annotatef(err, "parsing log msg")
	}

	t = InferYear(t)
	t = t.UTC()

	decreasedTimestamp := false

	if t.Before(lastTime) {
		// Time has decreased: this might happen if the previous log line had
		// a precise timestamp with microseconds, but the current line only has
		// a second precision. Then we just hackishly set the current timestamp
		// to be the same.
		t = lastTime
		decreasedTimestamp = true
	}

	ctxMap := map[string]string{}

	if false {
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

		// Another hack: often our messages contain a prefix like
		// "[ foo.bar.baz.info ]", but this is redundant since
		// "foo.bar.baz" is already present as a namespace, and "info"
		// is present as level_name. So we check if such redundant prefix
		// exists, and strip it.
		if strings.HasPrefix(msg, "[ ") {
			if idx := strings.Index(msg, " ] "); idx >= 0 {
				if msg[2:idx] == ctxMap["namespace"]+"."+ctxMap["level_name"] {
					// Prefix exists and is redundant, strip it.
					msg = msg[idx+3:]
				}
			}
		}
	}

	return &parseLineResult{
		time:               t,
		decreasedTimestamp: decreasedTimestamp,
		msg:                msg,
		ctxMap:             ctxMap,
	}, nil
}

func parseLineStructured(
	loc *time.Location, lastTime time.Time, sdmsg *parseSystemdMsgResult,
) (*parseLineResult, error) {
	tsStr := sdmsg.tsStr
	msg := sdmsg.msg

	var msgParsed struct {
		Fields map[string]interface{}
	}

	if err := json.Unmarshal([]byte(msg), &msgParsed); err != nil {
		return nil, errors.Trace(err)
	}

	ctxMap := make(map[string]string, len(msgParsed.Fields))
	for k, v := range msgParsed.Fields {
		var vStr string
		switch vTyped := v.(type) {
		case string:
			vStr = vTyped
		default:
			vStr = fmt.Sprintf("%v", v)
		}

		ctxMap[k] = vStr
	}

	if ts2Str, ok := ctxMap["time"]; ok {
		tsStr = ts2Str
		delete(ctxMap, "time")
	}

	// Now, tsStr contains the timestamp to parse, and msg contains
	// everything after that timestamp.

	t, err := time.ParseInLocation(syslogTimeLayout, tsStr, loc)
	if err != nil {
		return nil, errors.Annotatef(err, "parsing log msg")
	}

	t = InferYear(t)
	t = t.UTC()

	decreasedTimestamp := false

	if t.Before(lastTime) {
		// Time has decreased: this might happen if the previous log line had
		// a precise timestamp with microseconds, but the current line only has
		// a second precision. Then we just hackishly set the current timestamp
		// to be the same.
		t = lastTime
		decreasedTimestamp = true
	}

	if msg2, ok := ctxMap["msg"]; ok {
		msg = msg2
		delete(ctxMap, "msg")
	}

	return &parseLineResult{
		time:               t,
		decreasedTimestamp: decreasedTimestamp,
		msg:                msg,
		ctxMap:             ctxMap,
	}, nil
}
