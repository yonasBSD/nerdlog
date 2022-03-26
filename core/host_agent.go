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

	"github.com/juju/errors"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const connectionTimeout = 5 * time.Second

const queryLogsTimeLayout = "Jan-02-15:04"

type HostAgent struct {
	params HostAgentParams

	connectResCh chan hostConnectRes
	enqueueCmdCh chan hostCmd

	state HostAgentState

	conn *connCtx

	cmdQueue   []hostCmd
	curCmdCtx  *hostCmdCtx
	nextCmdIdx int
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

		state:        HostAgentStateDisconnected,
		enqueueCmdCh: make(chan hostCmd, 32),
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

		case line := <-ha.conn.getStdoutLinesCh():
			lastUpdTime = time.Now()

			// TODO: depending on state
			_ = line
			if ha.params.Config.Name == "my-host-01" {
				fmt.Println("rx:", line)
			}

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
						// TODO
						ha.sendCmdResp(nil, nil)
						ha.changeState(HostAgentStateConnectedIdle)

					default:
						panic(fmt.Sprintf("unhandled cmd %+v", ha.curCmdCtx.cmd))
					}
				}
			}

		case line := <-ha.conn.getStderrLinesCh():
			lastUpdTime = time.Now()

			// TODO maybe save somewhere for debugging
			if ha.params.Config.Name == "my-host-01" {
				fmt.Println("rxe:", line)
			}
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

	hostAddr := fmt.Sprintf("%s:%d", config.Hostname, config.Port)

	var sshClient *ssh.Client

	conf := getClientConfig(config.User)
	fmt.Printf("hey %+v %q\n", config, hostAddr)
	//fmt.Println("hey2", conn, conf)

	if true {
		// Use jumphost
		jumphost, err := getJumphostClient()
		if err != nil {
			res.err = errors.Annotatef(err, "getting jumphost client")
			return res
		}

		conn, err := dialWithTimeout(jumphost, "tcp", hostAddr, connectionTimeout)
		if err != nil {
			res.err = errors.Trace(err)
			return res
		}

		authConn, chans, reqs, err := ssh.NewClientConn(conn, hostAddr, conf)
		if err != nil {
			res.err = errors.Trace(err)
			return res
		}

		sshClient = ssh.NewClient(authConn, chans, reqs)
	} else {
		var err error
		sshClient, err = ssh.Dial("tcp", hostAddr, conf)
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

func (ha *HostAgent) EnqueueCmd(cmd hostCmd) {
	ha.enqueueCmdCh <- cmd
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

		ha.conn.stdinBuf.Write([]byte(`cat <<- 'EOF' > /var/tmp/query_logs.sh
#/bin/bash

cachefile=/tmp/query_logs_cache

logfile1=/var/log/syslog.1
logfile2=/var/log/syslog

positional_args=()

while [[ $# -gt 0 ]]; do
  case $1 in
    -f|--from)
      from="$2"
      shift # past argument
      shift # past value
      ;;
    -t|--to)
      to="$2"
      shift # past argument
      shift # past value
      ;;
    -u|--lines-until)
      lines_until="$2"
      shift # past argument
      shift # past value
      ;;
    -*|--*)
      echo "Unknown option $1" 1>&2
      exit 1
      ;;
    *)
      positional_args+=("$1") # save positional arg
      shift # past argument
      ;;
  esac
done

set -- "${positional_args[@]}" # restore positional parameters

user_pattern=$1

function refresh_cache { # {{{
  local lastnr=0
  local awknrplus="NR"

  # Add new entries to cache, if needed
  if [ -s $cachefile ]; then
    echo "caching new line numbers" 1>&2

    local typ="$(tail -n 1 $cachefile | cut -f1)"
    local lastts="$(tail -n 1 $cachefile | cut -f2)"
    local lastnr="$(tail -n 1 $cachefile | cut -f3)"
    local awknrplus="NR+$(( lastnr-1 ))"

    echo hey $lastts 1>&2
    echo hey2 $lastnr 1>&2
    #lastnr=$(( lastnr-1 ))

    # TODO: as one more optimization, we can store the size of the logfile1 in
    # the cache, so here we get this file size and below we don't cat it.
    local logfile1_numlines=0

    cat $logfile1 $logfile2 | tail -n +$((lastnr-logfile1_numlines)) | awk "BEGIN { lastts = \"$lastts\" }"'
  { curts = $1 "-" $2 "-" substr($3, 1, 5) }
  ( lastts != curts ) { print "idx\t" curts "\t" NR+'$(( lastnr-1 ))'; lastts = curts }
  ' - >> $cachefile
  else
    echo "caching all line numbers" 1>&2

    echo "prevlog_modtime	$(stat -c %y $logfile1)" > $cachefile

    cat $logfile1 | awk '
  { curts = $1 "-" $2 "-" substr($3, 1, 5) }
  ( lastts != curts ) { print "idx\t" curts "\t" NR; lastts = curts }
  END { print "prevlog_lines\t" NR }
  ' - >> $cachefile

    cat $logfile2 | awk '
  { curts = $1 "-" $2 "-" substr($3, 1, 5) }
  ( lastts != curts ) { print "idx\t" curts "\t" NR+'$(get_prevlog_lines_from_cache)'; lastts = curts }
  ' - >> $cachefile
  fi
} # }}}

function get_from_cache() { # {{{
  awk -F"\t" '$1 == "idx" && $2 == "'$1'" { print $3; exit }' $cachefile
} # }}}

function get_prevlog_lines_from_cache() { # {{{
  if ! awk -F"\t" 'BEGIN { found=0 } $1 == "prevlog_lines" { print $2; found = 1; exit } END { if (found == 0) { exit 1 } }' $cachefile ; then
    return 1
  fi
} # }}}

function get_prevlog_modtime_from_cache() { # {{{
  if ! awk -F"\t" 'BEGIN { found=0 } $1 == "prevlog_modtime" { print $2; found = 1; exit } END { if (found == 0) { exit 1 } }' $cachefile ; then
    return 1
  fi
} # }}}

if [[ "$from" != "" || "$to" != "" ]]; then
  # Check timestamp in the first line of /tmp/query_logs_cache, and if
  # $logfile1's modification time is newer, then delete whole cache
  logfile1_stored_modtime="$(get_prevlog_modtime_from_cache)"
  logfile1_cur_modtile=$(stat -c %y $logfile1)
  if [[ "$logfile1_stored_modtime" != "$logfile1_cur_modtile" ]]; then
    echo "logfile has changed: stored '$logfile1_stored_modtime', actual '$logfile1_cur_modtile'" 1>&2
    rm $cachefile
  fi

  if ! get_prevlog_lines_from_cache > /dev/null; then
    echo "broken cache file (no prevlog lines), deleting it" 1>&2
    rm $cachefile
  fi

  refresh_and_retry=0

  # First try to find it in cache without refreshing the cache

  # NOTE: as of now, it doesn't support a case when there were no messages
  # during whole minute at all. We just assume all our services do log
  # something at least once a minute.

  if [[ "$from" != "" ]]; then
    from_nr=$(get_from_cache $from)
    if [[ "$from_nr" == "" ]]; then
      echo "the from isn't found, gonna refresh the cache" 1>&2
      refresh_and_retry=1
    fi
  fi

  if [[ "$to" != "" ]]; then
    to_nr=$(get_from_cache $to)
    if [[ "$to_nr" == "" ]]; then
      echo "the to isn't found, gonna refresh the cache" 1>&2
      refresh_and_retry=1
    fi
  fi

  if [[ "$refresh_and_retry" == 1 ]]; then
    refresh_cache

    if [[ "$from" != "" ]]; then
      from_nr=$(get_from_cache $from)
      if [[ "$from_nr" == "" ]]; then
        echo "the from isn't found, will use the beginning" 1>&2
      fi
    fi

    if [[ "$to" != "" ]]; then
      to_nr=$(get_from_cache $to)
      if [[ "$to_nr" == "" ]]; then
        echo "the to isn't found, will use the end" 1>&2
      fi
    fi

  fi
fi

echo "from $from_nr to $to_nr" 1>&2

echo "scanning logs" 1>&2

awk_pattern=''
if [[ "$user_pattern" != "" ]]; then
  awk_pattern="!($user_pattern) {next}"
fi

lines_until_check=''
if [[ "$lines_until" != "" ]]; then
  lines_until_check="if (NR >= $lines_until) { next; }"
fi

awk_script='
BEGIN { curline=0; maxlines=100 }
'$awk_pattern'
{
  stats[$1 $2 "-" substr($3,1,5)]++;

  '$lines_until_check'

  lastlines[curline++] = $0;
  if (curline >= maxlines) {
    curline = 0;
  }

  next;
}

END {
  for (x in stats) {
    print "stats: " x " --> " stats[x]
  }

  for (i = 0; i < maxlines; i++) {
    ln = curline + i;
    if (ln >= maxlines) {
      ln -= maxlines;
    }

    print "line " i "(" ln ")" ": " lastlines[ln];
  }
}
'
logfiles="$logfile1 $logfile2"

if [[ "$from_nr" != "" ]]; then
  # Let's see if we need to check the $logfile1 at all
  prevlog_lines=$(get_prevlog_lines_from_cache)
  if [[ $(( prevlog_lines < from_nr )) == 1 ]]; then
    echo "Ignoring prev log file" 1>&2
    from_nr=$(( from_nr - prevlog_lines ))
    if [[ "$to_nr" != "" ]]; then
      to_nr=$(( to_nr - prevlog_lines ))
    fi
    logfiles="$logfile2"
  fi
fi

if [[ "$from_nr" == "" && "$to_nr" == "" ]]; then
  cat $logfiles | awk "$awk_script" - | sort
elif [[ "$from_nr" != "" && "$to_nr" == "" ]]; then
  cat $logfiles | tail -n +$from_nr | awk "$awk_script" - | sort
elif [[ "$from_nr" == "" && "$to_nr" != "" ]]; then
  cat $logfiles | head -n $to_nr | awk "$awk_script" - | sort
else
  cat $logfiles | tail -n +$from_nr | head -n $((to_nr - from_nr)) | awk "$awk_script" - | sort
fi
EOF
`))
		ha.conn.stdinBuf.Write([]byte("if [ $? == 0 ]; then echo 'bootstrap ok'; else echo 'bootstrap failed'; fi\n"))

	case cmdCtx.cmd.ping != nil:
		cmdCtx.pingCtx = &hostCmdCtxPing{}

		ha.conn.stdinBuf.Write([]byte("whoami\n"))

	case cmdCtx.cmd.queryLogs != nil:
		cmdCtx.queryLogsCtx = &hostCmdCtxQueryLogs{}

		parts := []string{
			"bash /var/tmp/query_logs.sh",
		}

		if !cmdCtx.cmd.queryLogs.from.IsZero() {
			parts = append(parts, "--from", cmdCtx.cmd.queryLogs.from.UTC().Format(queryLogsTimeLayout))
		}

		if !cmdCtx.cmd.queryLogs.to.IsZero() {
			parts = append(parts, "--to", cmdCtx.cmd.queryLogs.to.UTC().Format(queryLogsTimeLayout))
		}

		cmd := strings.Join(parts, " ") + "\n"
		fmt.Println("hey", ha.params.Config.Name, "cmd:", cmd)

		ha.conn.stdinBuf.Write([]byte(cmd))

	default:
		panic(fmt.Sprintf("invalid command %+v", cmdCtx.cmd))
	}

	ha.conn.stdinBuf.Write([]byte(fmt.Sprintf("echo 'command_done:%d'\n", cmdCtx.idx)))

	ha.changeState(HostAgentStateConnectedBusy)
}
