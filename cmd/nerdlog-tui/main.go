package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dimonomid/nerdlog/core"
	"github.com/rivo/tview"
)

// TODO: make multiple of them
const inputTimeLayout = "Jan02_15:04"

func main() {
	var hm *core.HostsManager
	var mainView *MainView

	app := tview.NewApplication()

	cmdCh := make(chan string, 8)
	go func() {
		for {
			cmd := <-cmdCh
			//mainView.ShowMessagebox("tmp", "Tmp", "Command: "+cmd, nil)

			parts := strings.Fields(cmd)
			if len(parts) == 0 {
				return
			}

			switch parts[0] {
			case "time":
				if len(parts) < 2 {
					mainView.ShowMessagebox("err", "Error", ":time requires an argument, like -5h", nil)
					return
				}

				from, err := parseAndInferTimeOrDur(inputTimeLayout, parts[1])
				if err != nil {
					mainView.ShowMessagebox("err", "Error", "invalid 'from' duration: "+err.Error(), nil)
					return
				}

				to := TimeOrDur{}

				if len(parts) >= 3 && parts[2] != "" {
					var err error
					to, err = parseAndInferTimeOrDur(inputTimeLayout, parts[2])
					if err != nil {
						mainView.ShowMessagebox("err", "Error", "invalid 'to' duration: "+err.Error(), nil)
						return
					}
				}

				mainView.SetTimeRange(from, to)
				mainView.DoQuery()

			default:
				mainView.ShowMessagebox("err", "Error", fmt.Sprintf("unknown command %q", parts[0]), nil)
			}
		}
	}()

	mainView = NewMainView(&MainViewParams{
		App: app,
		OnLogQuery: func(params core.QueryLogsParams) {
			hm.QueryLogs(params)
		},
		OnCmd: func(cmd string) {
			cmdCh <- cmd
		},
	})

	// Set default time range
	from, to := TimeOrDur{Dur: -1 * time.Hour}, TimeOrDur{}
	mainView.setTimeRange(from, to)

	hm = initHostsManager(mainView)

	fmt.Println("Starting UI ...")
	if err := app.SetRoot(mainView.GetUIPrimitive(), true).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		//sc.Close()
		//tc.Close()
		os.Exit(1)
	}

	// We end up here when the user quits the UI

	fmt.Println("")
	fmt.Println("Closing connections...")

	//sc.Close()
	//tc.Close()

	fmt.Println("Have a nice day.")
}

func initHostsManager(mainView *MainView) *core.HostsManager {
	updatesCh := make(chan core.HostsManagerUpdate, 128)
	var hm *core.HostsManager

	go func() {
		doneInitialQuery := false

		for {
			upd := <-updatesCh

			switch {
			case upd.State != nil:
				mainView.ApplyHMState(upd.State)
				if !doneInitialQuery && upd.State.Connected {
					mainView.DoQuery()
					doneInitialQuery = true
				}

			case upd.LogResp != nil:
				if len(upd.LogResp.Errs) > 0 {
					// TODO: include other errors too, not only the first one
					mainView.ShowMessagebox("err", "Log query error", upd.LogResp.Errs[0].Error(), nil)
					continue
				}
				mainView.ApplyLogs(upd.LogResp)

			default:
				panic("empty hosts manager update")
			}
		}
	}()

	hm = core.NewHostsManager(core.HostsManagerParams{
		ConfigHosts: makeConfigHosts(),
		UpdatesCh:   updatesCh,
	})

	return hm
}

/*
func main() {
	updatesCh := make(chan core.HostsManagerUpdate, 128)

	go func() {
		for {
			upd := <-updatesCh

			switch {
			case upd.State != nil:
				busyStr := "idle"
				if upd.State.Busy {
					busyStr = "busy"
				}

				fmt.Printf(
					"%s || Connected: %d/%d (idle %d, busy %d)\n",
					busyStr,
					len(upd.State.HostsByState[core.HostAgentStateConnectedIdle])+len(upd.State.HostsByState[core.HostAgentStateConnectedBusy]),
					upd.State.NumHosts,
					len(upd.State.HostsByState[core.HostAgentStateConnectedIdle]),
					len(upd.State.HostsByState[core.HostAgentStateConnectedBusy]),
				)

			case upd.LogResp != nil:
				resp := upd.LogResp
				keys := make([]int64, 0, len(resp.MinuteStats))
				for k := range resp.MinuteStats {
					keys = append(keys, k)
				}

				sort.Slice(keys, func(i, j int) bool {
					return keys[i] < keys[j]
				})

				fmt.Println("Log Response:")
				for _, seconds := range keys {
					item := resp.MinuteStats[seconds]

					t := time.Unix(seconds, 0)
					fmt.Printf("%s: %d\n", t, item.NumMsgs)
				}
				fmt.Println("------")

				for _, msg := range resp.Logs {
					fmt.Printf("%s: %s\n", msg.Time, msg.Msg)
				}

				fmt.Println("------")

			default:
				panic("empty hosts manager update")
			}
		}
	}()

	hm := core.NewHostsManager(core.HostsManagerParams{
		ConfigHosts: makeConfigHosts(),
		UpdatesCh:   updatesCh,
	})

	for {
		reader := bufio.NewReader(os.Stdin)
		// ReadString will block until the delimiter is entered
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("user read error", err)
			break
		}

		_ = input

		if input == "ping\n" {
			fmt.Println("sending pings...")
			hm.Ping()
		} else if input == "query\n" {
			fmt.Println("querying logs...")
			hm.QueryLogs(core.QueryLogsParams{
				From: time.Now().Add(-8 * time.Hour),
			})
		} else {
			fmt.Println("invalid comand")
		}
	}
}
*/

func makeConfigHosts() []core.ConfigHost {
	hosts := []core.ConfigHost{}

	for i := 0; i < 24; i++ {
		addr := fmt.Sprintf("my-host-%.2d:22", i+1)
		switch i + 1 {
		case 1:
			addr = "redacted:22"
		case 2:
			addr = "redacted:22"
		case 3:
			addr = "redacted:22"
		case 4:
			addr = "redacted:22"
		case 5:
			addr = "redacted:22"
		case 6:
			addr = "redacted:22"
		case 7:
			addr = "redacted:22"
		case 8:
			addr = "redacted:22"
		case 9:
			addr = "redacted:22"
		case 10:
			addr = "redacted:22"
		case 11:
			addr = "redacted:22"
		case 12:
			addr = "redacted:22"
		case 13:
			addr = "redacted:22"
		case 14:
			addr = "redacted:22"
		case 15:
			addr = "redacted:22"
		case 16:
			addr = "redacted:22"
		case 17:
			addr = "redacted:22"
		case 18:
			addr = "redacted:22"
		case 19:
			addr = "redacted:22"
		case 20:
			addr = "redacted:22"
		case 21:
			addr = "redacted:22"
		case 22:
			addr = "redacted:22"
		case 23:
			addr = "redacted:22"
		case 24:
			addr = "redacted:22"
		}

		hosts = append(hosts, core.ConfigHost{
			Name: fmt.Sprintf("my-host-%.2d", i+1),
			Addr: addr,
			User: "ubuntu",
		})
	}

	return hosts
}

/*
func main() {
	jumphost, err := getJumphostClient()
	if err != nil {
		panic(err.Error())
	}
	defer jumphost.Close()

	fmt.Println("jumphost ok", jumphost)

	// ---

	hostAddr := fmt.Sprintf("%s:%d", "my-host-01", 22) // TODO from config

	conn, err := dialWithTimeout(jumphost, "tcp", hostAddr, connectionTimeout)
	if err != nil {
		panic(err.Error())
	}

	conf := getClientConfig("ubuntu") // TODO from config

	authConn, chans, reqs, err := ssh.NewClientConn(conn, hostAddr, conf)
	if err != nil {
		panic(err.Error())
	}

	client := ssh.NewClient(authConn, chans, reqs)
	defer client.Close()

	fmt.Println("client ok", client)

	sess, err := client.NewSession()
	if err != nil {
		panic(err.Error())
	}

	defer sess.Close()

	fmt.Println("sess ok", sess)

	stdinBuf, err := sess.StdinPipe()
	if err != nil {
		panic(err.Error())
	}
	fmt.Println("stdin ok")

	stdoutBuf, err := sess.StdoutPipe()
	if err != nil {
		panic(err.Error())
	}
	fmt.Println("stdout ok")

	stderrBuf, err := sess.StderrPipe()
	if err != nil {
		panic(err.Error())
	}
	fmt.Println("stderr ok")

	err = sess.Shell()
	if err != nil {
		panic(err.Error())
	}
	fmt.Println("shell ok")

	go func() {
		for {
			b := make([]byte, 128)
			n, err := stdoutBuf.Read(b)
			if n > 0 {
				fmt.Print(string(b[:n]))
			}
			if err != nil {
				fmt.Println("stdin read error", err)
				break
			}
		}

		fmt.Println("stopped reading stdin")
	}()

	go func() {
		for {
			b := make([]byte, 128)
			n, err := stderrBuf.Read(b)
			if n > 0 {
				fmt.Print(string(b[:n]))
			}
			if err != nil {
				fmt.Println("stderr read error", err)
				break
			}
		}

		fmt.Println("stopped reading stderr")
	}()

	fmt.Println("accepting commands now")

	for {
		reader := bufio.NewReader(os.Stdin)
		// ReadString will block until the delimiter is entered
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("user read error", err)
			break
		}

		_ = input

		for i := 0; i < 1; i++ {
			cmd := fmt.Sprintf(`time bash /var/tmp/nerdlog_query.sh --from Mar-25-12:00 '/series_ids_string=\|523029\|/ && !/Activity monitor/'
`)
			stdinBuf.Write([]byte(cmd))
		}

		//stdinBuf.Write([]byte("ls /tmp\n"))
		//stdinBuf.Write([]byte(input))
		//fmt.Println("write1 ok")
		//stdinBuf.Write([]byte("ls /\n"))
		//fmt.Println("write2 ok")
	}

	fmt.Println("done")
}
*/

func parseAndInferTimeOrDur(layout, s string) (TimeOrDur, error) {
	t, err := ParseTimeOrDur(layout, s)
	if err != nil {
		return TimeOrDur{}, err
	}

	if t.IsAbsolute() {
		t.Time = core.InferYear(t.Time)
	}

	return t, nil
}
