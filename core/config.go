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
	LogSubjects map[string]ConfigLogSubject `yaml:"log_subjects"`
}

type ConfigLogSubject struct {
	// Name is an arbitrary string which will be included in log messages as the
	// "source" context tag.
	Name string `yaml:"name"`

	Host ConfigHost `yaml:"host"`
	// TODO: some jumphost config

	LogFileLast string `yaml:"log_file_last"`
	LogFilePrev string `yaml:"log_file_prev"`
}

type ConfigHost struct {
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
}

func (ch *ConfigLogSubject) Key() string {
	d, err := json.Marshal(ch)
	if err != nil {
		panic(err.Error())
	}

	return string(d)
}

// TODO: it should take a predefined config, to support globs
func parseConfigHost(s string) ([]*ConfigLogSubject, error) {
	parts, err := shellescape.Parse(s)
	if err != nil {
		return nil, errors.Trace(err)
	}

	var phost *parsedHost

	curFlag := ""
	for _, part := range parts {
		if curFlag == "" && len(part) > 0 && part[0] == '-' {
			curFlag = part
			continue
		}

		switch curFlag {
		case "-J", "--jumphost":
			return nil, errors.Errorf("Jumphost is not yet supported")

		case "":
			var err error
			phost, err = parseHostStr(part)
			if err != nil {
				return nil, errors.Annotatef(err, "parsing %q as a host", part)
			}
		default:
			return nil, errors.Errorf("invalid flag %s", curFlag)
		}
	}

	if phost == nil {
		return nil, errors.Errorf("no host specified in %q", s)
	}

	return []*ConfigLogSubject{
		{
			Name: s,

			Host: ConfigHost{
				Addr: phost.addr,
				User: phost.user,
			},

			LogFileLast: phost.logFileLast,
			LogFilePrev: phost.logFilePrev,
		},
	}, nil
}

type parsedHost struct {
	// addr is "host:port"
	addr string
	// user is the username to use
	user string

	logFileLast string
	logFilePrev string
}

func parseHostStr(s string) (*parsedHost, error) {
	// Parsing the host descriptor like
	// "user@hostname:/path/to/logfile:/path/to/logfile.1"

	// Parse user, if present
	username := ""
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

	port := ""
	if len(parts) >= 2 {
		port = parts[1]
	}
	if port == "" {
		port = "22"
	}

	logFileLast := ""
	if len(parts) >= 3 {
		logFileLast = parts[2]
	}
	if logFileLast == "" {
		logFileLast = "/var/log/syslog"
	}

	logFilePrev := ""
	if len(parts) >= 4 {
		logFilePrev = parts[3]
	}
	if logFilePrev == "" {
		logFilePrev = logFileLast + ".1"
	}

	if len(parts) > 4 {
		return nil, errors.Errorf("malformed host descriptor: too many colons")
	}

	return &parsedHost{
		addr:        fmt.Sprintf("%s:%s", hostname, port),
		user:        username,
		logFileLast: logFileLast,
		logFilePrev: logFilePrev,
	}, nil
}
