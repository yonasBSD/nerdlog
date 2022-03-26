package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/dimonomid/nerdlog/core"
)

func main() {
	hosts := []core.ConfigHost{}

	for i := 0; i < 24; i++ {
		hostname := fmt.Sprintf("my-host-%.2d", i+1)
		switch i + 1 {
		case 1:
			hostname = "redacted"
		case 2:
			hostname = "redacted"
		case 3:
			hostname = "redacted"
		case 4:
			hostname = "redacted"
		case 5:
			hostname = "redacted"
		case 6:
			hostname = "redacted"
		case 7:
			hostname = "redacted"
		case 8:
			hostname = "redacted"
		case 9:
			hostname = "redacted"
		case 10:
			hostname = "redacted"
		case 11:
			hostname = "redacted"
		case 12:
			hostname = "redacted"
		case 13:
			hostname = "redacted"
		case 14:
			hostname = "redacted"
		case 15:
			hostname = "redacted"
		case 16:
			hostname = "redacted"
		case 17:
			hostname = "redacted"
		case 18:
			hostname = "redacted"
		case 19:
			hostname = "redacted"
		case 20:
			hostname = "redacted"
		case 21:
			hostname = "redacted"
		case 22:
			hostname = "redacted"
		case 23:
			hostname = "redacted"
		case 24:
			hostname = "redacted"
		}

		hosts = append(hosts, core.ConfigHost{
			Name:     fmt.Sprintf("my-host-%.2d", i+1),
			Hostname: hostname,
			Port:     22,
			User:     "ubuntu",
		})
	}

	updatesCh := make(chan core.HostsManagerUpdate, 128)

	go func() {
		for {
			upd := <-updatesCh

			switch {
			case upd.State != nil:
				fmt.Printf("State: %+v\n", upd.State)

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
		ConfigHosts: hosts,
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
