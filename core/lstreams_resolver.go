package core

import (
	"fmt"
	"strings"

	"github.com/dimonomid/nerdlog/shellescape"
	"github.com/gobwas/glob"
	"github.com/juju/errors"
	"github.com/kevinburke/ssh_config"
)

type LStreamsResolver struct {
	params LStreamsResolverParams
}

type LStreamsResolverParams struct {
	// CurOSUser is the current OS username, it's used as the last resort when
	// determining the user for a particular host connection.
	CurOSUser string

	// ConfigLogStreams is the nerdlog-specific config, typically coming from
	// ~/.config/nerdlog/logstreams.yaml.
	ConfigLogStreams ConfigLogStreams

	// SSHConfig is the general SSH config, typically coming from ~/.ssh/config
	SSHConfig *ssh_config.Config
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
	// "host:port". The host must be a literal IP address, or a host name that
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

func (ls LogStream) LogFileLast() string {
	// LogFiles must contain at least a single item, so we don't check anything
	// here, and let it panic naturally if the invariant breaks due to some bug.
	return ls.LogFiles[0]
}

func (ls LogStream) LogFilePrev() (string, bool) {
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
func (r *LStreamsResolver) Resolve(lstreamsStr string) (map[string]LogStream, error) {
	lstreamsStr = strings.TrimSpace(lstreamsStr)

	parsedLogStreams := map[string]LogStream{}

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
func (r *LStreamsResolver) parseLogStreamSpecEntry(s string) ([]LogStream, error) {
	parts, err := shellescape.Parse(s)
	if err != nil {
		return nil, errors.Trace(err)
	}

	var plstream *parsedLStream
	var jhconf *ConfigHost
	var logFiles []string

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
				logFiles = append(logFiles, plstream.colonParts[0])
			}

			if len(plstream.colonParts) > 1 {
				logFiles = append(logFiles, plstream.colonParts[1])
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

	ret := []LogStream{
		{
			Name: s,

			Host: ConfigHost{
				Addr: fmt.Sprintf("%s:%s", plstream.hostname, plstream.port),
				User: plstream.user,
			},
			Jumphost: jhconf,

			LogFiles: logFiles,
		},
	}

	ret, err = expandFromLogStreamsConfig(ret, r.params.ConfigLogStreams)
	if err != nil {
		return nil, errors.Annotatef(err, "expanding from nerdlog config")
	}

	lsConfigFromSSHConfig, err := sshConfigToLSConfig(r.params.SSHConfig)
	if err != nil {
		return nil, errors.Annotatef(err, "parsing ssh config")
	}

	ret, err = expandFromLogStreamsConfig(ret, lsConfigFromSSHConfig)
	if err != nil {
		return nil, errors.Annotatef(err, "expanding from ssh config")
	}

	ret, err = setLogStreamsDefaults(ret, r.params.CurOSUser)
	if err != nil {
		return nil, errors.Annotatef(err, "setting defaults")
	}

	// Check if some of the items were clearly indended to be globs matching
	// something (those with asterisks in them), and didn't match anything.
	for _, ls := range ret {
		// TODO: would perhaps be useful to implement a function like IsValidDialAddress,
		// which checks a bunch of other things, but for now, a single asterisk check
		// will do.
		if strings.Contains(ls.Host.Addr, "*") {
			return nil, errors.Errorf("glob %q didn't match anything (having address %q)", s, ls.Host.Addr)
		}
	}

	return ret, nil
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

	port := ""
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
}

type ConfigLogStreamWKey struct {
	// Key is the key at which the corresponding ConfigLogStream was
	// stored in the ConfigLogStreams map.
	Key string

	ConfigLogStream
}

// expandFromLogStreamsConfig goes through each of the logstreams, and
// potentially expands every item as per the provided config.
func expandFromLogStreamsConfig(
	logStreams []LogStream,
	lsConfig ConfigLogStreams,
) ([]LogStream, error) {
	// If there's no config, cut it short.
	if lsConfig == nil {
		return logStreams, nil
	}

	var ret []LogStream

	for i, ls := range logStreams {
		var matchedConfigItems []*ConfigLogStreamWKey

		addr, err := parseAddr(ls.Host.Addr)
		if err != nil {
			return nil, errors.Annotatef(err, "logstream #%d, parsing address", i+1)
		}

		globPattern := addr.host
		matcher, err := glob.Compile(globPattern)
		if err != nil {
			return nil, errors.Annotatef(err, "logstream #%d, parsing hostname %q as a glob pattern", i+1, addr.host)
		}

		for _, key := range lsConfig.Keys() {
			if matcher.Match(key) {
				matchedConfigItems = append(matchedConfigItems, &ConfigLogStreamWKey{
					Key:             key,
					ConfigLogStream: lsConfig[key],
				})
			}
		}

		// If there's no match, just copy that logstream unchanged.
		if len(matchedConfigItems) == 0 {
			ret = append(ret, ls)
			continue
		}

		// There are some matches, so we need to expand things.
		for _, matchedItem := range matchedConfigItems {
			lsCopy := ls
			addrCopy := addr

			// Always override the name with the key from the config.
			lsCopy.Name = strings.Replace(lsCopy.Name, globPattern, matchedItem.Key, -1)

			// Overwrite the host address (since what we've had might be a glob):
			// either with the Hostname if it's specified explicitly, or if not, then
			// with the item key.
			if matchedItem.Hostname != "" {
				addrCopy.host = matchedItem.Hostname
			} else {
				addrCopy.host = matchedItem.Key
			}

			// Everything else we'll only override if it's not specified already.

			if addrCopy.port == "" {
				addrCopy.port = matchedItem.Port
			}

			if lsCopy.Host.User == "" {
				lsCopy.Host.User = matchedItem.User
			}

			if len(lsCopy.LogFiles) == 0 {
				lsCopy.LogFiles = matchedItem.LogFiles
			}

			lsCopy.Host.Addr = fmt.Sprintf("%s:%s", addrCopy.host, addrCopy.port)

			ret = append(ret, lsCopy)
		}
	}

	return ret, nil
}

// setLogStreamsDefaults goes through each of the logstreams, and fills in
// missing pieces for which it knows the defaults: port 22, user as the current
// OS user.
func setLogStreamsDefaults(
	logStreams []LogStream,
	osUser string,
) ([]LogStream, error) {
	ret := make([]LogStream, 0, len(logStreams))

	for i, ls := range logStreams {
		port, err := portFromAddr(ls.Host.Addr)
		if err != nil {
			return nil, errors.Annotatef(err, "logstream #%d, getting port", i+1)
		}

		if port == "" {
			ls.Host.Addr += "22"
		}

		if ls.Host.User == "" {
			ls.Host.User = osUser
		}

		if len(ls.LogFiles) == 0 {
			// Will be autodetected by the agent script.
			ls.LogFiles = append(ls.LogFiles, "auto")
		}

		if len(ls.LogFiles) == 1 {
			// Will be autodetected by the agent script.
			ls.LogFiles = append(ls.LogFiles, "auto")
		}

		ret = append(ret, ls)
	}

	return ret, nil
}

type parsedAddr struct {
	host string
	port string
}

func parseAddr(addr string) (parsedAddr, error) {
	parts := strings.Split(addr, ":")
	if len(parts) != 2 {
		return parsedAddr{}, errors.Errorf("not a valid addr %q, expected host:port", addr)
	}

	return parsedAddr{
		host: parts[0],
		port: parts[1],
	}, nil
}

// hostnameFromAddr takes an address like net.Dial takes, in the form of
// "host:port", and returns the host part.
func hostnameFromAddr(addr string) (string, error) {
	parts := strings.Split(addr, ":")
	if len(parts) != 2 {
		return "", errors.Errorf("not a valid addr %q, expected host:port", addr)
	}

	return parts[0], nil
}

// portFromAddr takes an address like net.Dial takes, in the form of
// "host:port", and returns the port part.
func portFromAddr(addr string) (string, error) {
	parts := strings.Split(addr, ":")
	if len(parts) != 2 {
		return "", errors.Errorf("not a valid addr %q, expected host:port", addr)
	}

	return parts[1], nil
}

func sshConfigToLSConfig(sshConfig *ssh_config.Config) (ConfigLogStreams, error) {
	if sshConfig == nil {
		return nil, nil
	}

	ret := make(ConfigLogStreams, len(sshConfig.Hosts))

	for _, host := range sshConfig.Hosts {
		if len(host.Patterns) == 0 {
			continue
		}

		name := host.Patterns[0].String()
		if name == "" {
			continue
		}

		// If it's a pattern, ignore it
		// (there might be valid use cases where we'd want to use them, but
		// not bothering for now)
		if strings.ContainsAny(name, "*?[]") {
			continue
		}

		hostname, _ := sshConfig.Get(name, "HostName")
		port, _ := sshConfig.Get(name, "Port")
		user, _ := sshConfig.Get(name, "User")

		if hostname == "" && port == "" && user == "" {
			// We can't get anything useful out of this entry anyway, so don't add it
			continue
		}

		ret[name] = ConfigLogStream{
			Hostname: hostname,
			Port:     port,
			User:     user,
		}
	}

	return ret, nil
}
