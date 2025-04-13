package core

import "sort"

type ConfigLogStreams map[string]ConfigLogStream

type ConfigLogStream struct {
	// HostAddr is the actual host to connect to.
	//
	// If empty, we'll resort to the ssh config, and if there's no info for that
	// host either, we'll try to get host addr from the key in
	// ConfigLogStreams.LogStreams.
	Hostname string `yaml:"hostname"`

	// Port is the actual port to connect to. If empty, the same overriding rules
	// apply.
	Port string `yaml:"port"`

	// User is the user to authenticate as. If empty, same overriding rules
	// apply.
	User string `yaml:"user"`

	// TODO: optional Jumphost configuration, also with addr and user.

	// LogFiles contains a list of files which are part of the logstream, like
	// ["/var/log/syslog", "/var/log/syslog.1"]. The [0]th item is the latest log
	// file [1]st is the previous one, etc.
	//
	// It must contain at least a single item, otherwise LogStream is invalid.
	LogFiles []string `yaml:"log_files"`
}

func (lss ConfigLogStreams) Keys() []string {
	keys := make([]string, 0, len(lss))
	for k := range lss {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return keys
}
