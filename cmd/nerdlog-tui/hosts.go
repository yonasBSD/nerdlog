package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dimonomid/nerdlog/core"
	"github.com/kevinburke/ssh_config"
)

func makeConfigHosts() []core.ConfigHost {
	if true {
		hosts := []core.ConfigHost{}

		f, _ := os.Open(filepath.Join(os.Getenv("HOME"), ".ssh", "dummy_config"))
		cfg, _ := ssh_config.Decode(f)
		for _, host := range cfg.Hosts {
			name := host.Patterns[0].String()
			hostName, err := cfg.Get(name, "HostName")
			if err != nil {
				continue
			}

			port, err := cfg.Get(name, "Port")
			if err != nil {
				continue
			}

			user, err := cfg.Get(name, "User")
			if err != nil {
				continue
			}

			if name == "" || hostName == "" || port == "" || user == "" {
				continue
			}

			hc := core.ConfigHost{
				Name: name,
				Addr: fmt.Sprintf("%s:%s", hostName, port),
				User: user,
			}

			hosts = append(hosts, hc)
		}

		return hosts
	} else {
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
}
