package core

import (
	"fmt"
	"os/user"
	"strings"

	"github.com/dimonomid/nerdlog/shellescape"
	"github.com/juju/errors"
)

type Config struct {
	LogStreams map[string]ConfigLogStream `yaml:"log_streams"`
}

type ConfigLogStream struct {
	// Name is an arbitrary string which will be included in log messages as the
	// "source" context tag; it must unique identify the ConfigLogStream.
	Name string `yaml:"name"`

	Host     ConfigHost  `yaml:"logstream"`
	Jumphost *ConfigHost `yaml:"jumphost"`

	LogFileLast string `yaml:"log_file_last"`
	LogFilePrev string `yaml:"log_file_prev"`
}

type ConfigHost struct {
	// Addr is the address to connect to, in the same format which is used by
	// net.Dial. To copy-paste some docs from net.Dial: the address has the form
	// "logstream:port". The logstream must be a literal IP address, or a logstream name that
	// can be resolved to IP addresses. The port must be a literal port number or
	// a service name.
	//
	// Examples: "golang.org:http", "192.0.2.1:http", "198.51.100.1:22".
	Addr string `yaml:"addr"`
	// User is the username to authenticate as.
	User string `yaml:"user"`
}

func (ch *ConfigHost) Key() string {
	return fmt.Sprintf("%s@%s", ch.Addr, ch.User)
}

// TODO: it should take a predefined config, to support globs
func parseConfigHost(s string) ([]*ConfigLogStream, error) {
	parts, err := shellescape.Parse(s)
	if err != nil {
		return nil, errors.Trace(err)
	}

	var plstream *parsedLStream
	var jhconf *ConfigHost
	var logFileLast, logFilePrev string

	curFlag := ""
	for _, part := range parts {
		if curFlag == "" && len(part) > 0 && part[0] == '-' {
			curFlag = part
			continue
		}

		switch curFlag {
		case "-J", "--jumphost":
			jhparsed, err := parseLStreamStr(part)
			if err != nil {
				return nil, errors.Annotatef(err, "parsing %q as a jumphost", part)
			}

			//if jhparsed.logFileLast != "" || jhparsed.logFilePrev != "" {
			//return nil, errors.Annotatef(err, "jumphost config shouldn't contain files")
			//}

			jhPort := jhparsed.port

			if len(jhparsed.colonParts) > 1 {
				return nil, errors.Errorf("parsing %q as a jumphost: too many colons", part)
			}

			jhconf = &ConfigHost{
				Addr: fmt.Sprintf("%s:%s", jhparsed.hostname, jhPort),
				User: jhparsed.user,
			}

		case "":
			var err error
			plstream, err = parseLStreamStr(part)
			if err != nil {
				return nil, errors.Annotatef(err, "parsing %q as a logstream", part)
			}

			if len(plstream.colonParts) > 0 {
				logFileLast = plstream.colonParts[0]
			} else {
				logFileLast = "/var/log/syslog"
			}

			if len(plstream.colonParts) > 1 {
				logFilePrev = plstream.colonParts[1]
			} else {
				logFilePrev = logFileLast + ".1"
			}

			if len(plstream.colonParts) > 2 {
				return nil, errors.Errorf("%q: too many colons", part)
			}
		default:
			return nil, errors.Errorf("invalid flag %s", curFlag)
		}

		curFlag = ""
	}

	if plstream == nil {
		return nil, errors.Errorf("no logstream specified in %q", s)
	}

	return []*ConfigLogStream{
		{
			Name: s,

			Host: ConfigHost{
				Addr: fmt.Sprintf("%s:%s", plstream.hostname, plstream.port),
				User: plstream.user,
			},
			Jumphost: jhconf,

			LogFileLast: logFileLast,
			LogFilePrev: logFilePrev,
		},
	}, nil
}

type parsedLStream struct {
	hostname string
	user     string
	port     string

	colonParts []string
}

func parseLStreamStr(s string) (*parsedLStream, error) {
	// Parsing the logstream descriptor like
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

	port := "22"
	colonParts := []string{}
	parts := strings.Split(s, ":")
	if len(parts) == 0 {
		return nil, errors.Errorf("no hostname")
	}

	if len(parts) > 1 {
		port = parts[1]
	}

	if len(parts) > 2 {
		colonParts = parts[2:]
	}

	return &parsedLStream{
		hostname:   parts[0],
		user:       username,
		port:       port,
		colonParts: colonParts,
	}, nil

	//hostname := parts[0]

	//port := ""
	//if len(parts) >= 2 {
	//port = parts[1]
	//}
	//if port == "" {
	//port = "22"
	//}

	//logFileLast := ""
	//if len(parts) >= 3 {
	//logFileLast = parts[2]
	//}
	//if logFileLast == "" {
	//logFileLast = "/var/log/syslog"
	//}

	//logFilePrev := ""
	//if len(parts) >= 4 {
	//logFilePrev = parts[3]
	//}
	//if logFilePrev == "" {
	//logFilePrev = logFileLast + ".1"
	//}

	//if len(parts) > 4 {
	//return nil, errors.Errorf("malformed logstream descriptor: too many colons")
	//}

	//return &parsedLStream{
	//addr:        fmt.Sprintf("%s:%s", hostname, port),
	//user:        username,
	//logFileLast: logFileLast,
	//logFilePrev: logFilePrev,
	//}, nil
}
