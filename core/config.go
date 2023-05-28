package core

import (
	"encoding/json"
	"fmt"
	"os/user"
	"strings"

	"github.com/dimonomid/nerdlog/shellescape"
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

	LogFileLast string `yaml:"logFileLast"`
	LogFilePrev string `yaml:"logFilePrev"`

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
	parts, err := shellescape.Parse(s)
	if err != nil {
		return nil, errors.Trace(err)
	}

	username := ""
	hostname := ""
	logFileLast := ""
	logFilePrev := ""
	port := "22"

	curFlag := ""
	for _, part := range parts {
		if curFlag == "" && len(part) > 0 && part[0] == '-' {
			curFlag = part
			continue
		}

		switch curFlag {
		case "-p", "--port":
			port = part

		case "":
			// Parsing the host descriptor like
			// "user@hostname:/path/to/logfile:/path/to/logfile.1"
			// Parse user, if present
			atIdx := strings.IndexRune(part, '@')
			if atIdx == 0 {
				return nil, errors.Errorf("username is empty")
			} else if atIdx > 0 {
				username = part[:atIdx]
				part = part[atIdx+1:]
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
			parts2 := strings.Split(part, ":")
			if len(parts2) == 0 {
				return nil, errors.Errorf("no hostname")
			}

			hostname = parts2[0]
			logFileLast = "/var/log/syslog"
			if len(parts2) >= 2 {
				logFileLast = parts2[1]
			}

			logFilePrev = logFileLast + ".1"
			if len(parts2) >= 3 {
				logFilePrev = parts2[2]
			}

			if len(parts2) > 3 {
				return nil, errors.Errorf("malformed host descriptor: too many colons")
			}

		default:
			return nil, errors.Errorf("invalid flag %s", curFlag)
		}
	}

	return []*ConfigHost{
		{
			Name: s,
			Addr: fmt.Sprintf("%s:%s", hostname, port),
			User: username,

			LogFileLast: logFileLast,
			LogFilePrev: logFilePrev,
		},
	}, nil
}
