package core

import (
	"encoding/json"
	"fmt"
	"os/user"
	"strings"

	"github.com/juju/errors"
)

type Config struct {
	Hosts map[string]ConfigHost `yaml:"hosts"`
}

type ConfigHost struct {
	// Name is an arbitrary string which will be included in log messages as the
	// "source" context tag.
	Name string `yaml:"name"`

	// Addr is the address to connect to, in the same format which is used by
	// net.Dial. To copy-paste some docs from net.Dial: the address has the form
	// "host:port". The host must be a literal IP address, or a host name that
	// can be resolved to IP addresses. The port must be a literal port number or
	// a service name.
	//
	// Examples: "golang.org:http", "192.0.2.1:http", "198.51.100.1:22".
	Addr string `yaml:"addr"`
	// User is the username to authenticate as.
	User string `yaml:"user"`

	LogFile1 string `yaml:"logFile1"`
	LogFile2 string `yaml:"logFile2"`

	// TODO: some jumphost config
}

func (ch *ConfigHost) Key() string {
	d, err := json.Marshal(ch)
	if err != nil {
		panic(err.Error())
	}

	return string(d)
}

// TODO: it should take a predefined config, to support globs
func parseConfigHost(s string) ([]*ConfigHost, error) {
	// Parse user, if present
	var username string
	atIdx := strings.IndexRune(s, '@')
	if atIdx == 0 {
		return nil, errors.Errorf("username is empty")
	} else if atIdx > 0 {
		username = s[:atIdx]
		s = s[atIdx+1:]
	}

	if username == "" {
		u, err := user.Current()
		if err != nil {
			return nil, errors.Trace(err)
		}

		username = u.Username
	}

	// Split string by ":", expecting at most 3 parts:
	// "hostname:/path/to/logfile:/path/to/logfile.1"
	parts := strings.Split(s, ":")
	if len(parts) == 0 {
		return nil, errors.Errorf("no hostname")
	}

	hostname := parts[0]
	logFile1 := "/var/log/syslog"
	if len(parts) >= 2 {
		logFile1 = parts[1]
	}

	logFile2 := logFile1 + ".1"
	if len(parts) >= 3 {
		logFile2 = parts[2]
	}

	if len(parts) > 3 {
		return nil, errors.Errorf("malformed host descriptor: too many colons")
	}

	// TODO: make port customizable
	port := "22"

	return []*ConfigHost{
		{
			Name: s,
			Addr: fmt.Sprintf("%s:%s", hostname, port),
			User: username,

			LogFile1: logFile1,
			LogFile2: logFile2,
		},
	}, nil
}
