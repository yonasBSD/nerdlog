package core

import (
	"bytes"
	_ "embed"
	"fmt"
	"testing"

	"github.com/dimonomid/ssh_config"
	"github.com/stretchr/testify/assert"
)

//go:embed resolver_testdata/ssh_config_1
var testSSHConfig1Str []byte
var testSSHConfig1 *ssh_config.Config

func init() {
	buf := bytes.NewBuffer(testSSHConfig1Str)
	var err error
	testSSHConfig1, err = ssh_config.Decode(buf, false)
	if err != nil {
		panic(fmt.Sprintf("embedded ssh_config_1 is broken: %s", err.Error()))
	}
}

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

	"baz-01": ConfigLogStream{
		LogFiles: []string{"/from/nerdlog/config/bazlog"},
	},

	"baz-02": ConfigLogStream{
		LogFiles: []string{"/from/nerdlog/config/bazlog"},
	},

	"realhost.com": ConfigLogStream{
		User: "user-from-nerdlog-config",
	},
})

type resolverTestCase struct {
	// name is the name of the test case
	name string
	// osUser is the current OS username
	osUser string

	configLogStreams ConfigLogStreams
	sshConfig        *ssh_config.Config

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
		SSHConfig:        tc.sshConfig,
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
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "myserver.com:22",
								User: "osuser",
							},
						},
					},
					LogFiles: []string{"auto", "auto"},
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
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "myserver.com:22",
								User: "myuser",
							},
						},
					},
					LogFiles: []string{"auto", "auto"},
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
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "myserver.com:777",
								User: "myuser",
							},
						},
					},
					LogFiles: []string{"auto", "auto"},
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
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "myserver.com:777",
								User: "osuser",
							},
						},
					},
					LogFiles: []string{"auto", "auto"},
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
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "myserver.com:22",
								User: "myuser",
							},
						},
					},
					LogFiles: []string{"/var/log/syslog", "auto"},
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
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "myserver.com:22",
								User: "myuser",
							},
						},
					},
					LogFiles: []string{"/var/log/auth.log", "auto"},
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
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "myserver.com:22",
								User: "myuser",
							},
						},
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
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host1.com:22",
								User: "osuser",
							},
						},
					},
					LogFiles: []string{"auto", "auto"},
				},
				"host2.com": {
					Name: "host2.com",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host2.com:22",
								User: "osuser",
							},
						},
					},
					LogFiles: []string{"auto", "auto"},
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
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "foo.com:2200",
								User: "alice",
							},
						},
					},
					LogFiles: []string{"/a.log", "/b.log"},
				},
				"bob@bar.com": {
					Name: "bob@bar.com",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "bar.com:22",
								User: "bob",
							},
						},
					},
					LogFiles: []string{"auto", "auto"},
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
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-from-nerdlog-config-01.com:1001",
								User: "user-from-nerdlog-config-01",
							},
						},
					},
					LogFiles: []string{"/from/nerdlog/config/mylog_1", "auto"},
				},
				"myhost-02": {
					Name: "myhost-02",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-from-nerdlog-config-02.com:1002",
								User: "user-from-nerdlog-config-02",
							},
						},
					},
					LogFiles: []string{"/from/nerdlog/config/mylog_1", "/from/nerdlog/config/mylog_2"},
				},
				"myhost-03": {
					Name: "myhost-03",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-from-nerdlog-config-03.com:1003",
								User: "user-from-nerdlog-config-03",
							},
						},
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
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-from-nerdlog-config-01.com:1001",
								User: "user-from-nerdlog-config-01",
							},
						},
					},
					LogFiles: []string{"/from/nerdlog/config/mylog_1", "auto"},
				},
				"myhost-02": {
					Name: "myhost-02",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-from-nerdlog-config-02.com:1002",
								User: "user-from-nerdlog-config-02",
							},
						},
					},
					LogFiles: []string{"/from/nerdlog/config/mylog_1", "/from/nerdlog/config/mylog_2"},
				},
				"myhost-03": {
					Name: "myhost-03",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-from-nerdlog-config-03.com:1003",
								User: "user-from-nerdlog-config-03",
							},
						},
					},
					LogFiles: []string{"/from/nerdlog/config/mylog_1", "/from/nerdlog/config/mylog_2"},
				},

				"foo-01": {
					Name: "foo-01",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-foo-from-nerdlog-config-01.com:2001",
								User: "user-foo-from-nerdlog-config-01",
							},
						},
					},
					LogFiles: []string{"/from/nerdlog/config/foolog", "auto"},
				},
				"foo-02": {
					Name: "foo-02",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-foo-from-nerdlog-config-02.com:2002",
								User: "user-foo-from-nerdlog-config-02",
							},
						},
					},
					LogFiles: []string{"/from/nerdlog/config/foolog", "auto"},
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
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-bar-from-nerdlog-config-01.com:22",
								User: "user-bar-from-nerdlog-config-01",
							},
						},
					},
					LogFiles: []string{"auto", "auto"},
				},
				"bar-02": {
					Name: "bar-02",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-bar-from-nerdlog-config-02.com:22",
								User: "user-bar-from-nerdlog-config-02",
							},
						},
					},
					LogFiles: []string{"auto", "auto"},
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
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-foo-from-nerdlog-config-01.com:123",
								User: "user-foo-from-nerdlog-config-01",
							},
						},
					},
					LogFiles: []string{"/from/nerdlog/config/foolog", "auto"},
				},
				"foo-02:123": {
					Name: "foo-02:123",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-foo-from-nerdlog-config-02.com:123",
								User: "user-foo-from-nerdlog-config-02",
							},
						},
					},
					LogFiles: []string{"/from/nerdlog/config/foolog", "auto"},
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
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-foo-from-nerdlog-config-01.com:2001",
								User: "customuser",
							},
						},
					},
					LogFiles: []string{"/from/nerdlog/config/foolog", "auto"},
				},
				"customuser@foo-02": {
					Name: "customuser@foo-02",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-foo-from-nerdlog-config-02.com:2002",
								User: "customuser",
							},
						},
					},
					LogFiles: []string{"/from/nerdlog/config/foolog", "auto"},
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
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-foo-from-nerdlog-config-01.com:2001",
								User: "user-foo-from-nerdlog-config-01",
							},
						},
					},
					LogFiles: []string{"/var/log/custom", "auto"},
				},
				"foo-02::/var/log/custom": {
					Name: "foo-02::/var/log/custom",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-foo-from-nerdlog-config-02.com:2002",
								User: "user-foo-from-nerdlog-config-02",
							},
						},
					},
					LogFiles: []string{"/var/log/custom", "auto"},
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
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-foo-from-nerdlog-config-01.com:2001",
								User: "user-foo-from-nerdlog-config-01",
							},
						},
					},
					LogFiles: []string{"/var/log/custom", "/var/log/custom_prev"},
				},
				"foo-02::/var/log/custom:/var/log/custom_prev": {
					Name: "foo-02::/var/log/custom:/var/log/custom_prev",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-foo-from-nerdlog-config-02.com:2002",
								User: "user-foo-from-nerdlog-config-02",
							},
						},
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
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-foo-from-nerdlog-config-01.com:444",
								User: "customuser",
							},
						},
					},
					LogFiles: []string{"/var/log/custom", "/var/log/custom_prev"},
				},
				"customuser@foo-02:444:/var/log/custom:/var/log/custom_prev": {
					Name: "customuser@foo-02:444:/var/log/custom:/var/log/custom_prev",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-foo-from-nerdlog-config-02.com:444",
								User: "customuser",
							},
						},
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
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-foo-from-nerdlog-config-01.com:2001",
								User: "user-foo-from-nerdlog-config-01",
							},
						},
					},
					LogFiles: []string{"/from/nerdlog/config/foolog", "auto"},
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
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-foo-from-nerdlog-config-01.com:2001",
								User: "customuser",
							},
						},
					},
					LogFiles: []string{"/from/nerdlog/config/foolog", "auto"},
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
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-foo-from-nerdlog-config-01.com:2001",
								User: "user-foo-from-nerdlog-config-01",
							},
						},
					},
					LogFiles: []string{"/from/nerdlog/config/foolog", "auto"},
				},
				"foo-02": {
					Name: "foo-02",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-foo-from-nerdlog-config-02.com:2002",
								User: "user-foo-from-nerdlog-config-02",
							},
						},
					},
					LogFiles: []string{"/from/nerdlog/config/foolog", "auto"},
				},

				"foo-01::/var/log/custom": {
					Name: "foo-01::/var/log/custom",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-foo-from-nerdlog-config-01.com:2001",
								User: "user-foo-from-nerdlog-config-01",
							},
						},
					},
					LogFiles: []string{"/var/log/custom", "auto"},
				},
				"foo-02::/var/log/custom": {
					Name: "foo-02::/var/log/custom",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-foo-from-nerdlog-config-02.com:2002",
								User: "user-foo-from-nerdlog-config-02",
							},
						},
					},
					LogFiles: []string{"/var/log/custom", "auto"},
				},
			},
		},

		{
			name:   "real host, hostname is not overridden",
			osUser: "osuser",

			configLogStreams: testConfigLogStreams1,
			input:            "realhost.com",

			wantStreams: map[string]LogStream{
				"realhost.com": {
					Name: "realhost.com",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "realhost.com:22",
								User: "user-from-nerdlog-config",
							},
						},
					},
					LogFiles: []string{"auto", "auto"},
				},
			},
		},

		{
			name:   "single glob, logfiles from nerdlog config",
			osUser: "osuser",

			configLogStreams: testConfigLogStreams1,

			input: "baz-*",

			wantStreams: map[string]LogStream{
				"baz-01": {
					Name: "baz-01",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "baz-01:22",
								User: "osuser",
							},
						},
					},
					LogFiles: []string{"/from/nerdlog/config/bazlog", "auto"},
				},
				"baz-02": {
					Name: "baz-02",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "baz-02:22",
								User: "osuser",
							},
						},
					},
					LogFiles: []string{"/from/nerdlog/config/bazlog", "auto"},
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

func TestLStreamsResolverGlobOnlySSHConfig(t *testing.T) {
	tests := []resolverTestCase{
		{
			name:   "single glob, everything is taken from ssh config",
			osUser: "osuser",

			sshConfig: testSSHConfig1,
			input:     "sshfoo-*",

			wantStreams: map[string]LogStream{
				"sshfoo-01": {
					Name: "sshfoo-01",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-foo-from-ssh-config-01.com:3001",
								User: "user-foo-from-ssh-config-01",
							},
						},
					},
					LogFiles: []string{"auto", "auto"},
				},
				"sshfoo-02": {
					Name: "sshfoo-02",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-foo-from-ssh-config-02.com:3002",
								User: "user-foo-from-ssh-config-02",
							},
						},
					},
					LogFiles: []string{"auto", "auto"},
				},
			},
		},

		{
			name:   "two globs, everything is taken from ssh config",
			osUser: "osuser",

			sshConfig: testSSHConfig1,
			input:     "sshfoo-*, sshbar-*",

			wantStreams: map[string]LogStream{
				"sshfoo-01": {
					Name: "sshfoo-01",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-foo-from-ssh-config-01.com:3001",
								User: "user-foo-from-ssh-config-01",
							},
						},
					},
					LogFiles: []string{"auto", "auto"},
				},
				"sshfoo-02": {
					Name: "sshfoo-02",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-foo-from-ssh-config-02.com:3002",
								User: "user-foo-from-ssh-config-02",
							},
						},
					},
					LogFiles: []string{"auto", "auto"},
				},
				"sshbar-01": {
					Name: "sshbar-01",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-bar-from-ssh-config-01.com:3001",
								User: "user-bar-from-ssh-config-01",
							},
						},
					},
					LogFiles: []string{"auto", "auto"},
				},
				"sshbar-02": {
					Name: "sshbar-02",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-bar-from-ssh-config-02.com:3002",
								User: "user-bar-from-ssh-config-02",
							},
						},
					},
					LogFiles: []string{"auto", "auto"},
				},
			},
		},

		{
			name:   "single glob, log file is from the input, everything else is from ssh config",
			osUser: "osuser",

			sshConfig: testSSHConfig1,
			input:     "sshfoo-*::/var/log/auth.log",

			wantStreams: map[string]LogStream{
				"sshfoo-01::/var/log/auth.log": {
					Name: "sshfoo-01::/var/log/auth.log",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-foo-from-ssh-config-01.com:3001",
								User: "user-foo-from-ssh-config-01",
							},
						},
					},
					LogFiles: []string{"/var/log/auth.log", "auto"},
				},
				"sshfoo-02::/var/log/auth.log": {
					Name: "sshfoo-02::/var/log/auth.log",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-foo-from-ssh-config-02.com:3002",
								User: "user-foo-from-ssh-config-02",
							},
						},
					},
					LogFiles: []string{"/var/log/auth.log", "auto"},
				},
			},
		},

		{
			name:   "single glob, exact match",
			osUser: "osuser",

			sshConfig: testSSHConfig1,
			input:     "sshfoo-02",

			wantStreams: map[string]LogStream{
				"sshfoo-02": {
					Name: "sshfoo-02",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-foo-from-ssh-config-02.com:3002",
								User: "user-foo-from-ssh-config-02",
							},
						},
					},
					LogFiles: []string{"auto", "auto"},
				},
			},
		},

		{
			name:   "single glob, exact match, host is the same",
			osUser: "osuser",

			sshConfig: testSSHConfig1,
			input:     "sshrealhost.com",

			wantStreams: map[string]LogStream{
				"sshrealhost.com": {
					Name: "sshrealhost.com",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "sshrealhost.com:4001",
								User: "user-from-ssh-config",
							},
						},
					},
					LogFiles: []string{"auto", "auto"},
				},
			},
		},

		{
			name:   "single glob, no port in ssh config",
			osUser: "osuser",

			sshConfig: testSSHConfig1,
			input:     "sshnoport-*",

			wantStreams: map[string]LogStream{
				"sshnoport-01": {
					Name: "sshnoport-01",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-noport-from-ssh-config-01.com:22",
								User: "user-noport-from-ssh-config-01",
							},
						},
					},
					LogFiles: []string{"auto", "auto"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runResolverTestCase(t, tt)
		})
	}
}

func TestLStreamsResolverGlobBothNerdlogAndSSHConfigs(t *testing.T) {
	tests := []resolverTestCase{
		{
			name:   "single glob, everything is taken from nerdlog config, even though it exists in ssh too",
			osUser: "osuser",

			configLogStreams: testConfigLogStreams1,
			sshConfig:        testSSHConfig1,

			input: "foo-*",

			wantStreams: map[string]LogStream{
				"foo-01": {
					Name: "foo-01",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-foo-from-nerdlog-config-01.com:2001",
								User: "user-foo-from-nerdlog-config-01",
							},
						},
					},
					LogFiles: []string{"/from/nerdlog/config/foolog", "auto"},
				},
				"foo-02": {
					Name: "foo-02",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-foo-from-nerdlog-config-02.com:2002",
								User: "user-foo-from-nerdlog-config-02",
							},
						},
					},
					LogFiles: []string{"/from/nerdlog/config/foolog", "auto"},
				},
			},
		},

		{
			name:   "single glob, taken most taken from nerdlog config, port from ssh config",
			osUser: "osuser",

			configLogStreams: testConfigLogStreams1,
			sshConfig:        testSSHConfig1,

			input: "bar-*",

			wantStreams: map[string]LogStream{
				"bar-01": {
					Name: "bar-01",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-bar-from-nerdlog-config-01.com:6001",
								User: "user-bar-from-nerdlog-config-01",
							},
						},
					},
					LogFiles: []string{"auto", "auto"},
				},
				"bar-02": {
					Name: "bar-02",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-bar-from-nerdlog-config-02.com:6002",
								User: "user-bar-from-nerdlog-config-02",
							},
						},
					},
					LogFiles: []string{"auto", "auto"},
				},
			},
		},

		{
			name:   "single glob, logfiles from nerdlog config, everything else from ssh config",
			osUser: "osuser",

			configLogStreams: testConfigLogStreams1,
			sshConfig:        testSSHConfig1,

			input: "baz-*",

			wantStreams: map[string]LogStream{
				"baz-01": {
					Name: "baz-01",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-baz-from-ssh-config-01.com:7001",
								User: "user-baz-from-ssh-config-01",
							},
						},
					},
					LogFiles: []string{"/from/nerdlog/config/bazlog", "auto"},
				},
				"baz-02": {
					Name: "baz-02",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "host-baz-from-ssh-config-02.com:7002",
								User: "user-baz-from-ssh-config-02",
							},
						},
					},
					LogFiles: []string{"/from/nerdlog/config/bazlog", "auto"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runResolverTestCase(t, tt)
		})
	}
}

func TestLStreamsResolverLocalhost(t *testing.T) {
	tests := []resolverTestCase{
		{
			name:   "single localhost entry",
			osUser: "osuser",
			input:  "localhost",
			wantStreams: map[string]LogStream{
				"localhost": {
					Name: "localhost",
					Transport: ConfigLogStreamShellTransport{
						Localhost: &ConfigLogStreamShellTransportLocalhost{},
					},
					LogFiles: []string{"auto", "auto"},
				},
			},
		},
		{
			name:   "localhost with user, which is ignored",
			osUser: "osuser",
			input:  "myuser@localhost",
			wantStreams: map[string]LogStream{
				"myuser@localhost": {
					Name: "myuser@localhost",
					Transport: ConfigLogStreamShellTransport{
						Localhost: &ConfigLogStreamShellTransportLocalhost{},
					},
					LogFiles: []string{"auto", "auto"},
				},
			},
		},
		{
			name:   "hostname with user and port, which are ignored",
			osUser: "osuser",
			input:  "myuser@localhost:777",
			wantStreams: map[string]LogStream{
				"myuser@localhost:777": {
					Name: "myuser@localhost:777",
					Transport: ConfigLogStreamShellTransport{
						Localhost: &ConfigLogStreamShellTransportLocalhost{},
					},
					LogFiles: []string{"auto", "auto"},
				},
			},
		},
		{
			name:   "hostname with port, which is ignored",
			osUser: "osuser",
			input:  "localhost:777",
			wantStreams: map[string]LogStream{
				"localhost:777": {
					Name: "localhost:777",
					Transport: ConfigLogStreamShellTransport{
						Localhost: &ConfigLogStreamShellTransportLocalhost{},
					},
					LogFiles: []string{"auto", "auto"},
				},
			},
		},
		{
			name:   "hostname with user, port, and log file; user and port are ignored",
			osUser: "osuser",
			input:  "myuser@localhost:22:/var/log/syslog",
			wantStreams: map[string]LogStream{
				"myuser@localhost:22:/var/log/syslog": {
					Name: "myuser@localhost:22:/var/log/syslog",
					Transport: ConfigLogStreamShellTransport{
						Localhost: &ConfigLogStreamShellTransportLocalhost{},
					},
					LogFiles: []string{"/var/log/syslog", "auto"},
				},
			},
		},
		{
			name:   "hostname with user, port, and different log file; user and port are ignored",
			osUser: "osuser",
			input:  "myuser@localhost:22:/var/log/auth.log",
			wantStreams: map[string]LogStream{
				"myuser@localhost:22:/var/log/auth.log": {
					Name: "myuser@localhost:22:/var/log/auth.log",
					Transport: ConfigLogStreamShellTransport{
						Localhost: &ConfigLogStreamShellTransportLocalhost{},
					},
					LogFiles: []string{"/var/log/auth.log", "auto"},
				},
			},
		},
		{
			name:   "hostname with user, port, and two log files; user and port are ignored",
			osUser: "osuser",
			input:  "myuser@localhost:22:/var/log/mylog_last:/var/log/mylog_prev",
			wantStreams: map[string]LogStream{
				"myuser@localhost:22:/var/log/mylog_last:/var/log/mylog_prev": {
					Name: "myuser@localhost:22:/var/log/mylog_last:/var/log/mylog_prev",
					Transport: ConfigLogStreamShellTransport{
						Localhost: &ConfigLogStreamShellTransportLocalhost{},
					},
					LogFiles: []string{"/var/log/mylog_last", "/var/log/mylog_prev"},
				},
			},
		},
		{
			name:   "127.0.0.1 still goes via ssh",
			osUser: "osuser",
			input:  "127.0.0.1",
			wantStreams: map[string]LogStream{
				"127.0.0.1": {
					Name: "127.0.0.1",
					Transport: ConfigLogStreamShellTransport{
						SSH: &ConfigLogStreamShellTransportSSH{
							Host: ConfigHost{
								Addr: "127.0.0.1:22",
								User: "osuser",
							},
						},
					},
					LogFiles: []string{"auto", "auto"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runResolverTestCase(t, tt)
		})
	}
}
