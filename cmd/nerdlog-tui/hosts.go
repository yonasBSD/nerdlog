package main

import (
	"github.com/dimonomid/nerdlog/core"
)

// TODO: use it when we support configs
func makeConfigHosts() []core.ConfigHost {
	return nil

	//if true {
	//hosts := []core.ConfigHost{}

	//f, _ := os.Open(filepath.Join(os.Getenv("HOME"), ".ssh", "config"))
	//cfg, _ := ssh_config.Decode(f)
	//for _, host := range cfg.Hosts {
	//name := host.Patterns[0].String()
	//hostName, err := cfg.Get(name, "HostName")
	//if err != nil {
	//continue
	//}

	//port, err := cfg.Get(name, "Port")
	//if err != nil {
	//continue
	//}

	//user, err := cfg.Get(name, "User")
	//if err != nil {
	//continue
	//}

	//if name == "" || hostName == "" || port == "" || user == "" {
	//continue
	//}

	//hc := core.ConfigHost{
	//Name: name,
	//Addr: fmt.Sprintf("%s:%s", hostName, port),
	//User: user,
	//}

	//hosts = append(hosts, hc)
	//}

	//return hosts
	//} else {
	//hosts := []core.ConfigHost{}

	//hosts = append(hosts, core.ConfigHost{
	//Name: fmt.Sprintf("dummynode-01:22"),
	//Addr: "127.0.0.1:22",
	//User: "ubuntu",
	//})

	//return hosts
	//}
}
