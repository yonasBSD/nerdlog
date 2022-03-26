package main

import (
	"fmt"
	"time"

	"github.com/dimonomid/slog/core"
)

func main() {
	hosts := []core.ConfigHost{}

	// TODO: wtf it works with 22 but doesn't with 24
	for i := 0; i < 24; i++ {
		hosts = append(hosts, core.ConfigHost{
			Name:     fmt.Sprintf("my-host-%.2d", i+1),
			Hostname: fmt.Sprintf("my-host-%.2d", i+1),
			Port:     22,
			User:     "ubuntu",
		})
	}

	hm := core.NewHostsManager(core.HostsManagerParams{
		ConfigHosts: hosts,
	})

	_ = hm
	time.Sleep(1 * time.Minute)
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
			cmd := fmt.Sprintf(`time bash /var/tmp/dmitrii_log.sh --from Mar-25-12:00 '/series_ids_string=\|523029\|/ && !/Activity monitor/'
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
