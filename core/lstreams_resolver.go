package core

import (
	"fmt"
	"strings"

	"github.com/dimonomid/nerdlog/shellescape"
	"github.com/juju/errors"
)

type LStreamsResolver struct {
	params LStreamsResolverParams
}

type LStreamsResolverParams struct {
	// CurOSUser is the current OS username, it's used as the last resort when
	// determining the user for a particular host connection.
	CurOSUser string

	// TODO: add ssh config and nerdlog config here, which will help with the resolving.
}

func NewLStreamsResolver(params LStreamsResolverParams) *LStreamsResolver {
	return &LStreamsResolver{
		params: params,
	}
}

type LogStream struct {
	// Name is an arbitrary string which will be included in log messages as the
	// "source" context tag; it must uniquely identify the LogStream.
	Name string

	Host     ConfigHost
	Jumphost *ConfigHost

	// LogFiles contains a list of files which are part of the logstream, like
	// ["/var/log/syslog", "/var/log/syslog.1"]. The [0]th item is the latest log
	// file [1]st is the previous one, etc.
	//
	// It must contain at least a single item, otherwise LogStream is invalid.
	LogFiles []string
}

type ConfigHost struct {
	// Addr is the address to connect to, in the same format which is used by
	// net.Dial. To copy-paste some docs from net.Dial: the address has the form
	// "logstream:port". The logstream must be a literal IP address, or a logstream name that
	// can be resolved to IP addresses. The port must be a literal port number or
	// a service name.
	//
	// Examples: "golang.org:http", "192.0.2.1:http", "198.51.100.1:22".
	Addr string
	// User is the username to authenticate as.
	User string
}

func (ch *ConfigHost) Key() string {
	return fmt.Sprintf("%s@%s", ch.Addr, ch.User)
}

func (ls *LogStream) LogFileLast() string {
	// LogFiles must contain at least a single item, so we don't check anything
	// here, and let it panic naturally if the invariant breaks due to some bug.
	return ls.LogFiles[0]
}

func (ls *LogStream) LogFilePrev() (string, bool) {
	if len(ls.LogFiles) >= 2 {
		return ls.LogFiles[1], true
	}

	return "", false
}

// Resolve parses the given logstream spec, and returns the mapping from
// LogStream.Name to the corresponding LogStream. Examples of logstream spec are:
//
// - "myuser@myserver.com:22:/var/log/syslog"
// - "myuser@myserver.com:22"
// - "myuser@myserver.com"
// - "myserver.com"
func (r *LStreamsResolver) Resolve(lstreamsStr string) (map[string]*LogStream, error) {
	lstreamsStr = strings.TrimSpace(lstreamsStr)

	parsedLogStreams := map[string]*LogStream{}

	// Special case for an empty input: it's allowed and just results in no
	// logstreams.
	if lstreamsStr == "" {
		return parsedLogStreams, nil
	}

	// TODO: when json is supported, splitting by commas will need to be improved.
	parts := strings.Split(lstreamsStr, ",")
	for i, part := range parts {
		part = strings.TrimSpace(part)

		if part == "" {
			return nil, errors.Errorf("entry #%d is empty", i+1)
		}

		cfs, err := r.parseLogStreamSpecEntry(part)
		if err != nil {
			return nil, errors.Annotatef(err, "parsing entry #%d (%s)", i+1, part)
		}

		for _, ch := range cfs {
			key := ch.Name

			if _, exists := parsedLogStreams[key]; exists {
				return nil, errors.Errorf("the logstream %s is present at least twice", key)
			}

			parsedLogStreams[key] = ch
		}

		//matcher, err := glob.Compile(part)
		//if err != nil {
		//return errors.Annotatef(err, "pattern %q", part)
		//}

		//numMatchedPart := 0

		//for _, hc := range lsman.params.ConfigHosts {
		//if !matcher.MatchString(hc.Name) {
		//continue
		//}

		//matchingHANames[hc.Name] = struct{}{}
		//numMatchedPart++
		//}

		//if numMatchedPart == 0 {
		//return errors.Errorf("%q didn't match anything", part)
		//}
	}

	return parsedLogStreams, nil
}

// parseLogStreamSpecEntry parses a single logstream spec entry like
// "myuser@myserver.com:22:/var/log/syslog", or "myserver.com", or
// "myserver-*", and returns the corresponding LogStream-s. Note that the spec
// might contain a glob, in which case we might return more than 1 LogStream.
// If the glob didn't match anything, an error is returned.
//
// TODO: it should take a predefined config, to support globs
func (r *LStreamsResolver) parseLogStreamSpecEntry(s string) ([]*LogStream, error) {
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
			jhparsed, err := r.parseLStreamStr(part)
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
			plstream, err = r.parseLStreamStr(part)
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

	return []*LogStream{
		{
			Name: s,

			Host: ConfigHost{
				Addr: fmt.Sprintf("%s:%s", plstream.hostname, plstream.port),
				User: plstream.user,
			},
			Jumphost: jhconf,

			LogFiles: []string{logFileLast, logFilePrev},
		},
	}, nil
}

type parsedLStream struct {
	hostname string
	user     string
	port     string

	colonParts []string
}

func (r *LStreamsResolver) parseLStreamStr(s string) (*parsedLStream, error) {
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
		if r.params.CurOSUser == "" {
			return nil, errors.Errorf("username is not provided, and CurOSUser is also empty")
		}

		username = r.params.CurOSUser
	}

	port := "22"
	colonParts := []string{}
	parts := strings.Split(s, ":")
	if len(parts) == 0 || parts[0] == "" {
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
