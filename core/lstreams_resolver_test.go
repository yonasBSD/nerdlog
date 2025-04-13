package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type resolverTestCase struct {
	// name is the name of the test case
	name string
	// osUser is the current OS username
	osUser string

	// input is the logstream spec string that we're feeding to Resolve()
	input string

	wantStreams map[string]*LogStream
	wantErr     string
}

func runResolverTestCase(t *testing.T, tc resolverTestCase) {
	t.Helper()

	resolver := NewLStreamsResolver(LStreamsResolverParams{
		CurOSUser: tc.osUser,
	})

	gotStreams, err := resolver.Resolve(tc.input)

	if tc.wantErr != "" {
		assert.EqualError(t, err, tc.wantErr)
	} else {
		assert.NoError(t, err, "unexpected error")
		assert.Equal(t, tc.wantStreams, gotStreams)
	}
}

func TestLStreamsResolverSingleEntry(t *testing.T) {
	tests := []resolverTestCase{
		{
			name:   "simple hostname only",
			osUser: "osuser",
			input:  "myserver.com",
			wantStreams: map[string]*LogStream{
				"myserver.com": {
					Name: "myserver.com",
					Host: ConfigHost{
						Addr: "myserver.com:22",
						User: "osuser",
					},
					LogFiles: []string{"/var/log/syslog", "/var/log/syslog.1"},
				},
			},
		},
		{
			name:   "hostname with user",
			osUser: "osuser",
			input:  "myuser@myserver.com",
			wantStreams: map[string]*LogStream{
				"myuser@myserver.com": {
					Name: "myuser@myserver.com",
					Host: ConfigHost{
						Addr: "myserver.com:22",
						User: "myuser",
					},
					LogFiles: []string{"/var/log/syslog", "/var/log/syslog.1"},
				},
			},
		},
		{
			name:   "hostname with user and port",
			osUser: "osuser",
			input:  "myuser@myserver.com:777",
			wantStreams: map[string]*LogStream{
				"myuser@myserver.com:777": {
					Name: "myuser@myserver.com:777",
					Host: ConfigHost{
						Addr: "myserver.com:777",
						User: "myuser",
					},
					LogFiles: []string{"/var/log/syslog", "/var/log/syslog.1"},
				},
			},
		},
		{
			name:   "hostname with port",
			osUser: "osuser",
			input:  "myserver.com:777",
			wantStreams: map[string]*LogStream{
				"myserver.com:777": {
					Name: "myserver.com:777",
					Host: ConfigHost{
						Addr: "myserver.com:777",
						User: "osuser",
					},
					LogFiles: []string{"/var/log/syslog", "/var/log/syslog.1"},
				},
			},
		},
		{
			name:   "hostname with user, port, and log file",
			osUser: "osuser",
			input:  "myuser@myserver.com:22:/var/log/syslog",
			wantStreams: map[string]*LogStream{
				"myuser@myserver.com:22:/var/log/syslog": {
					Name: "myuser@myserver.com:22:/var/log/syslog",
					Host: ConfigHost{
						Addr: "myserver.com:22",
						User: "myuser",
					},
					LogFiles: []string{"/var/log/syslog", "/var/log/syslog.1"},
				},
			},
		},
		{
			name:   "hostname with user, port, and different log file",
			osUser: "osuser",
			input:  "myuser@myserver.com:22:/var/log/auth.log",
			wantStreams: map[string]*LogStream{
				"myuser@myserver.com:22:/var/log/auth.log": {
					Name: "myuser@myserver.com:22:/var/log/auth.log",
					Host: ConfigHost{
						Addr: "myserver.com:22",
						User: "myuser",
					},
					LogFiles: []string{"/var/log/auth.log", "/var/log/auth.log.1"},
				},
			},
		},
		{
			name:   "hostname with user, port, and two log files",
			osUser: "osuser",
			input:  "myuser@myserver.com:22:/var/log/mylog_last:/var/log/mylog_prev",
			wantStreams: map[string]*LogStream{
				"myuser@myserver.com:22:/var/log/mylog_last:/var/log/mylog_prev": {
					Name: "myuser@myserver.com:22:/var/log/mylog_last:/var/log/mylog_prev",
					Host: ConfigHost{
						Addr: "myserver.com:22",
						User: "myuser",
					},
					LogFiles: []string{"/var/log/mylog_last", "/var/log/mylog_prev"},
				},
			},
		},
		{
			name:        "empty string is allowed",
			osUser:      "myuser",
			input:       "",
			wantStreams: map[string]*LogStream{},
		},
		{
			name:        "empty string with whitespaces is allowed",
			osUser:      "myuser",
			input:       "",
			wantStreams: map[string]*LogStream{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runResolverTestCase(t, tt)
		})
	}
}

func TestLStreamsResolverMultipleEntries(t *testing.T) {
	tests := []resolverTestCase{
		{
			name:   "two hosts with defaults",
			osUser: "osuser",
			input:  "host1.com,host2.com",
			wantStreams: map[string]*LogStream{
				"host1.com": {
					Name: "host1.com",
					Host: ConfigHost{
						Addr: "host1.com:22",
						User: "osuser",
					},
					LogFiles: []string{"/var/log/syslog", "/var/log/syslog.1"},
				},
				"host2.com": {
					Name: "host2.com",
					Host: ConfigHost{
						Addr: "host2.com:22",
						User: "osuser",
					},
					LogFiles: []string{"/var/log/syslog", "/var/log/syslog.1"},
				},
			},
		},
		{
			name:   "mixed full and partial formats",
			osUser: "osuser",
			input:  "alice@foo.com:2200:/a.log:/b.log, bob@bar.com",
			wantStreams: map[string]*LogStream{
				"alice@foo.com:2200:/a.log:/b.log": {
					Name: "alice@foo.com:2200:/a.log:/b.log",
					Host: ConfigHost{
						Addr: "foo.com:2200",
						User: "alice",
					},
					LogFiles: []string{"/a.log", "/b.log"},
				},
				"bob@bar.com": {
					Name: "bob@bar.com",
					Host: ConfigHost{
						Addr: "bar.com:22",
						User: "bob",
					},
					LogFiles: []string{"/var/log/syslog", "/var/log/syslog.1"},
				},
			},
		},
		{
			name:    "second entry is empty",
			osUser:  "osuser",
			input:   "alice@foo.com:2200:/a.log:/b.log, , bob@bar.com",
			wantErr: "entry #2 is empty",
		},
		{
			name:    "error in second entry",
			osUser:  "osuser",
			input:   "valid.com,myuser@",
			wantErr: "parsing entry #2 (myuser@): parsing \"myuser@\" as a logstream: no hostname",
		},
		{
			name:    "empty input with comma",
			osUser:  "osuser",
			input:   ",",
			wantErr: "entry #1 is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runResolverTestCase(t, tt)
		})
	}
}
