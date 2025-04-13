package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var testConfigLogStreams1 = ConfigLogStreams(map[string]ConfigLogStream{
	"myhost-01": ConfigLogStream{
		Hostname: "host-from-nerdlog-config-01.com",
		Port:     "1001",
		User:     "user-from-nerdlog-config-01",
		LogFiles: []string{"/from/nerdlog/config/mylog_1"},
	},
	"myhost-02": ConfigLogStream{
		Hostname: "host-from-nerdlog-config-02.com",
		Port:     "1002",
		User:     "user-from-nerdlog-config-02",
		LogFiles: []string{"/from/nerdlog/config/mylog_1", "/from/nerdlog/config/mylog_2"},
	},
	"myhost-03": ConfigLogStream{
		Hostname: "host-from-nerdlog-config-03.com",
		Port:     "1003",
		User:     "user-from-nerdlog-config-03",
		LogFiles: []string{"/from/nerdlog/config/mylog_1", "/from/nerdlog/config/mylog_2"},
	},

	"foo-01": ConfigLogStream{
		Hostname: "host-foo-from-nerdlog-config-01.com",
		Port:     "2001",
		User:     "user-foo-from-nerdlog-config-01",
		LogFiles: []string{"/from/nerdlog/config/foolog"},
	},
	"foo-02": ConfigLogStream{
		Hostname: "host-foo-from-nerdlog-config-02.com",
		Port:     "2002",
		User:     "user-foo-from-nerdlog-config-02",
		LogFiles: []string{"/from/nerdlog/config/foolog"},
	},

	"bar-01": ConfigLogStream{
		Hostname: "host-bar-from-nerdlog-config-01.com",
		User:     "user-bar-from-nerdlog-config-01",
	},
	"bar-02": ConfigLogStream{
		Hostname: "host-bar-from-nerdlog-config-02.com",
		User:     "user-bar-from-nerdlog-config-02",
	},
})

type resolverTestCase struct {
	// name is the name of the test case
	name string
	// osUser is the current OS username
	osUser string

	configLogStreams ConfigLogStreams

	// input is the logstream spec string that we're feeding to Resolve()
	input string

	wantStreams map[string]LogStream
	wantErr     string
}

func runResolverTestCase(t *testing.T, tc resolverTestCase) {
	t.Helper()

	resolver := NewLStreamsResolver(LStreamsResolverParams{
		CurOSUser:        tc.osUser,
		ConfigLogStreams: tc.configLogStreams,
	})

	gotStreams, err := resolver.Resolve(tc.input)

	if tc.wantErr != "" {
		assert.EqualError(t, err, tc.wantErr)
	} else {
		assert.NoError(t, err, "unexpected error")
		assert.Equal(t, tc.wantStreams, gotStreams)
	}
}

func TestLStreamsResolverSingleEntryNoGlob(t *testing.T) {
	tests := []resolverTestCase{
		{
			name:   "simple hostname only",
			osUser: "osuser",
			input:  "myserver.com",
			wantStreams: map[string]LogStream{
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
			wantStreams: map[string]LogStream{
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
			wantStreams: map[string]LogStream{
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
			wantStreams: map[string]LogStream{
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
			wantStreams: map[string]LogStream{
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
			wantStreams: map[string]LogStream{
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
			wantStreams: map[string]LogStream{
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
			wantStreams: map[string]LogStream{},
		},
		{
			name:        "empty string with whitespaces is allowed",
			osUser:      "myuser",
			input:       "",
			wantStreams: map[string]LogStream{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runResolverTestCase(t, tt)
		})
	}
}

func TestLStreamsResolverMultipleEntriesNoGlob(t *testing.T) {
	tests := []resolverTestCase{
		{
			name:   "two hosts with defaults",
			osUser: "osuser",
			input:  "host1.com,host2.com",
			wantStreams: map[string]LogStream{
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
			wantStreams: map[string]LogStream{
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

func TestLStreamsResolverGlobOnlyNerdlogConfig(t *testing.T) {
	tests := []resolverTestCase{
		{
			name:   "single glob, everything is taken from nerdlog config",
			osUser: "osuser",

			configLogStreams: testConfigLogStreams1,
			input:            "myhost-*",

			wantStreams: map[string]LogStream{
				"myhost-01": {
					Name: "myhost-01",
					Host: ConfigHost{
						Addr: "host-from-nerdlog-config-01.com:1001",
						User: "user-from-nerdlog-config-01",
					},
					LogFiles: []string{"/from/nerdlog/config/mylog_1", "/from/nerdlog/config/mylog_1.1"},
				},
				"myhost-02": {
					Name: "myhost-02",
					Host: ConfigHost{
						Addr: "host-from-nerdlog-config-02.com:1002",
						User: "user-from-nerdlog-config-02",
					},
					LogFiles: []string{"/from/nerdlog/config/mylog_1", "/from/nerdlog/config/mylog_2"},
				},
				"myhost-03": {
					Name: "myhost-03",
					Host: ConfigHost{
						Addr: "host-from-nerdlog-config-03.com:1003",
						User: "user-from-nerdlog-config-03",
					},
					LogFiles: []string{"/from/nerdlog/config/mylog_1", "/from/nerdlog/config/mylog_2"},
				},
			},
		},
		{
			name:   "two globs, everything is taken from nerdlog config",
			osUser: "osuser",

			configLogStreams: testConfigLogStreams1,
			input:            "myhost-*, foo-*",

			wantStreams: map[string]LogStream{
				"myhost-01": {
					Name: "myhost-01",
					Host: ConfigHost{
						Addr: "host-from-nerdlog-config-01.com:1001",
						User: "user-from-nerdlog-config-01",
					},
					LogFiles: []string{"/from/nerdlog/config/mylog_1", "/from/nerdlog/config/mylog_1.1"},
				},
				"myhost-02": {
					Name: "myhost-02",
					Host: ConfigHost{
						Addr: "host-from-nerdlog-config-02.com:1002",
						User: "user-from-nerdlog-config-02",
					},
					LogFiles: []string{"/from/nerdlog/config/mylog_1", "/from/nerdlog/config/mylog_2"},
				},
				"myhost-03": {
					Name: "myhost-03",
					Host: ConfigHost{
						Addr: "host-from-nerdlog-config-03.com:1003",
						User: "user-from-nerdlog-config-03",
					},
					LogFiles: []string{"/from/nerdlog/config/mylog_1", "/from/nerdlog/config/mylog_2"},
				},

				"foo-01": {
					Name: "foo-01",
					Host: ConfigHost{
						Addr: "host-foo-from-nerdlog-config-01.com:2001",
						User: "user-foo-from-nerdlog-config-01",
					},
					LogFiles: []string{"/from/nerdlog/config/foolog", "/from/nerdlog/config/foolog.1"},
				},
				"foo-02": {
					Name: "foo-02",
					Host: ConfigHost{
						Addr: "host-foo-from-nerdlog-config-02.com:2002",
						User: "user-foo-from-nerdlog-config-02",
					},
					LogFiles: []string{"/from/nerdlog/config/foolog", "/from/nerdlog/config/foolog.1"},
				},
			},
		},

		{
			name:   "single glob, everything is taken from nerdlog config, but port and logfiles are defaults",
			osUser: "osuser",

			configLogStreams: testConfigLogStreams1,
			input:            "bar-*",

			wantStreams: map[string]LogStream{
				"bar-01": {
					Name: "bar-01",
					Host: ConfigHost{
						Addr: "host-bar-from-nerdlog-config-01.com:22",
						User: "user-bar-from-nerdlog-config-01",
					},
					LogFiles: []string{"/var/log/syslog", "/var/log/syslog.1"},
				},
				"bar-02": {
					Name: "bar-02",
					Host: ConfigHost{
						Addr: "host-bar-from-nerdlog-config-02.com:22",
						User: "user-bar-from-nerdlog-config-02",
					},
					LogFiles: []string{"/var/log/syslog", "/var/log/syslog.1"},
				},
			},
		},

		{
			name:   "one glob, port is overridden by the input",
			osUser: "osuser",

			configLogStreams: testConfigLogStreams1,
			input:            "foo-*:123",

			wantStreams: map[string]LogStream{
				"foo-01:123": {
					Name: "foo-01:123",
					Host: ConfigHost{
						Addr: "host-foo-from-nerdlog-config-01.com:123",
						User: "user-foo-from-nerdlog-config-01",
					},
					LogFiles: []string{"/from/nerdlog/config/foolog", "/from/nerdlog/config/foolog.1"},
				},
				"foo-02:123": {
					Name: "foo-02:123",
					Host: ConfigHost{
						Addr: "host-foo-from-nerdlog-config-02.com:123",
						User: "user-foo-from-nerdlog-config-02",
					},
					LogFiles: []string{"/from/nerdlog/config/foolog", "/from/nerdlog/config/foolog.1"},
				},
			},
		},

		{
			name:   "one glob, user is overridden by the input",
			osUser: "osuser",

			configLogStreams: testConfigLogStreams1,
			input:            "customuser@foo-*",

			wantStreams: map[string]LogStream{
				"customuser@foo-01": {
					Name: "customuser@foo-01",
					Host: ConfigHost{
						Addr: "host-foo-from-nerdlog-config-01.com:2001",
						User: "customuser",
					},
					LogFiles: []string{"/from/nerdlog/config/foolog", "/from/nerdlog/config/foolog.1"},
				},
				"customuser@foo-02": {
					Name: "customuser@foo-02",
					Host: ConfigHost{
						Addr: "host-foo-from-nerdlog-config-02.com:2002",
						User: "customuser",
					},
					LogFiles: []string{"/from/nerdlog/config/foolog", "/from/nerdlog/config/foolog.1"},
				},
			},
		},

		{
			name:   "one glob, first logfile is overridden by the input, second is inferred",
			osUser: "osuser",

			configLogStreams: testConfigLogStreams1,
			input:            "foo-*::/var/log/custom",

			wantStreams: map[string]LogStream{
				"foo-01::/var/log/custom": {
					Name: "foo-01::/var/log/custom",
					Host: ConfigHost{
						Addr: "host-foo-from-nerdlog-config-01.com:2001",
						User: "user-foo-from-nerdlog-config-01",
					},
					LogFiles: []string{"/var/log/custom", "/var/log/custom.1"},
				},
				"foo-02::/var/log/custom": {
					Name: "foo-02::/var/log/custom",
					Host: ConfigHost{
						Addr: "host-foo-from-nerdlog-config-02.com:2002",
						User: "user-foo-from-nerdlog-config-02",
					},
					LogFiles: []string{"/var/log/custom", "/var/log/custom.1"},
				},
			},
		},

		{
			name:   "one glob, both logfiles are overridden by the input",
			osUser: "osuser",

			configLogStreams: testConfigLogStreams1,
			input:            "foo-*::/var/log/custom:/var/log/custom_prev",

			wantStreams: map[string]LogStream{
				"foo-01::/var/log/custom:/var/log/custom_prev": {
					Name: "foo-01::/var/log/custom:/var/log/custom_prev",
					Host: ConfigHost{
						Addr: "host-foo-from-nerdlog-config-01.com:2001",
						User: "user-foo-from-nerdlog-config-01",
					},
					LogFiles: []string{"/var/log/custom", "/var/log/custom_prev"},
				},
				"foo-02::/var/log/custom:/var/log/custom_prev": {
					Name: "foo-02::/var/log/custom:/var/log/custom_prev",
					Host: ConfigHost{
						Addr: "host-foo-from-nerdlog-config-02.com:2002",
						User: "user-foo-from-nerdlog-config-02",
					},
					LogFiles: []string{"/var/log/custom", "/var/log/custom_prev"},
				},
			},
		},

		{
			name:   "one glob, everything is overridden by the input",
			osUser: "osuser",

			configLogStreams: testConfigLogStreams1,
			input:            "customuser@foo-*:444:/var/log/custom:/var/log/custom_prev",

			wantStreams: map[string]LogStream{
				"customuser@foo-01:444:/var/log/custom:/var/log/custom_prev": {
					Name: "customuser@foo-01:444:/var/log/custom:/var/log/custom_prev",
					Host: ConfigHost{
						Addr: "host-foo-from-nerdlog-config-01.com:444",
						User: "customuser",
					},
					LogFiles: []string{"/var/log/custom", "/var/log/custom_prev"},
				},
				"customuser@foo-02:444:/var/log/custom:/var/log/custom_prev": {
					Name: "customuser@foo-02:444:/var/log/custom:/var/log/custom_prev",
					Host: ConfigHost{
						Addr: "host-foo-from-nerdlog-config-02.com:444",
						User: "customuser",
					},
					LogFiles: []string{"/var/log/custom", "/var/log/custom_prev"},
				},
			},
		},

		{
			name:   "exact match without globs",
			osUser: "osuser",

			configLogStreams: testConfigLogStreams1,
			input:            "foo-01",

			wantStreams: map[string]LogStream{
				"foo-01": {
					Name: "foo-01",
					Host: ConfigHost{
						Addr: "host-foo-from-nerdlog-config-01.com:2001",
						User: "user-foo-from-nerdlog-config-01",
					},
					LogFiles: []string{"/from/nerdlog/config/foolog", "/from/nerdlog/config/foolog.1"},
				},
			},
		},

		{
			name:   "exact match without globs, user is taken from the input",
			osUser: "osuser",

			configLogStreams: testConfigLogStreams1,
			input:            "customuser@foo-01",

			wantStreams: map[string]LogStream{
				"customuser@foo-01": {
					Name: "customuser@foo-01",
					Host: ConfigHost{
						Addr: "host-foo-from-nerdlog-config-01.com:2001",
						User: "customuser",
					},
					LogFiles: []string{"/from/nerdlog/config/foolog", "/from/nerdlog/config/foolog.1"},
				},
			},
		},

		{
			name:   "different files from the same hosts",
			osUser: "osuser",

			configLogStreams: testConfigLogStreams1,
			input:            "foo-*, foo-*::/var/log/custom",

			wantStreams: map[string]LogStream{
				"foo-01": {
					Name: "foo-01",
					Host: ConfigHost{
						Addr: "host-foo-from-nerdlog-config-01.com:2001",
						User: "user-foo-from-nerdlog-config-01",
					},
					LogFiles: []string{"/from/nerdlog/config/foolog", "/from/nerdlog/config/foolog.1"},
				},
				"foo-02": {
					Name: "foo-02",
					Host: ConfigHost{
						Addr: "host-foo-from-nerdlog-config-02.com:2002",
						User: "user-foo-from-nerdlog-config-02",
					},
					LogFiles: []string{"/from/nerdlog/config/foolog", "/from/nerdlog/config/foolog.1"},
				},

				"foo-01::/var/log/custom": {
					Name: "foo-01::/var/log/custom",
					Host: ConfigHost{
						Addr: "host-foo-from-nerdlog-config-01.com:2001",
						User: "user-foo-from-nerdlog-config-01",
					},
					LogFiles: []string{"/var/log/custom", "/var/log/custom.1"},
				},
				"foo-02::/var/log/custom": {
					Name: "foo-02::/var/log/custom",
					Host: ConfigHost{
						Addr: "host-foo-from-nerdlog-config-02.com:2002",
						User: "user-foo-from-nerdlog-config-02",
					},
					LogFiles: []string{"/var/log/custom", "/var/log/custom.1"},
				},
			},
		},

		{
			name:   "glob doesn't match anything",
			osUser: "osuser",

			configLogStreams: testConfigLogStreams1,
			input:            "mismatching-*",

			wantErr: "parsing entry #1 (mismatching-*): glob \"mismatching-*\" didn't match anything (having address \"mismatching-*:22\")",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runResolverTestCase(t, tt)
		})
	}
}
