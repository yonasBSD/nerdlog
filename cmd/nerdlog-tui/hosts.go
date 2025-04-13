package main

import (
	"github.com/dimonomid/nerdlog/core"
)

// TODO: use it when we support configs
func makeConfigHosts() []core.ConfigHost {
	return nil

	//if true {
	//lstreams := []core.ConfigHost{}

	//f, _ := os.Open(filepath.Join(os.Getenv("HOME"), ".ssh", "config"))
	//cfg, _ := ssh_config.Decode(f)
	//for _, logstream := range cfg.Hosts {
	//name := logstream.Patterns[0].String()
	//lstreamName, err := cfg.Get(name, "HostName")
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

	//if name == "" || lstreamName == "" || port == "" || user == "" {
	//continue
	//}

	//hc := core.ConfigHost{
	//Name: name,
	//Addr: fmt.Sprintf("%s:%s", lstreamName, port),
	//User: user,
	//}

	//lstreams = append(lstreams, hc)
	//}

	//return lstreams
	//} else {
	//lstreams := []core.ConfigHost{}

	//lstreams = append(lstreams, core.ConfigHost{
	//Name: fmt.Sprintf("dummynode-01:22"),
	//Addr: "127.0.0.1:22",
	//User: "ubuntu",
	//})

	//return lstreams
	//}
}
