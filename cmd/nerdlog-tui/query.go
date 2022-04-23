package main

import (
	"github.com/dimonomid/nerdlog/shellescape"
	"github.com/juju/errors"
)

// QueryFull contains everything that defines a query: the hosts filter, time range,
// and the query to filter logs.
type QueryFull struct {
	HostsFilter string
	Time        string
	Query       string
}

var execName = "nerdlog"

// numShellParts defines how many shell parts should be in the
// shell-command-marshalled form. It looks like this:
//
//   nerdlog --hosts <value> --time <value> --query <value>
//
// Therefore, there are 7 parts.
var numShellParts = 1 + 3*2

func (qf *QueryFull) MarshalShellCmd() string {
	parts := qf.MarshalShellCmdParts()
	return shellescape.Escape(parts)
}

func (qf *QueryFull) UnmarshalShellCmd(cmd string) error {
	parts, err := shellescape.Parse(cmd)
	if err != nil {
		return errors.Trace(err)
	}

	if err := qf.UnmarshalShellCmdParts(parts); err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (qf *QueryFull) MarshalShellCmdParts() []string {
	parts := make([]string, 0, numShellParts)

	parts = append(parts, execName)
	parts = append(parts, "--hosts", qf.HostsFilter)
	parts = append(parts, "--time", qf.Time)
	parts = append(parts, "--query", qf.Query)

	return parts
}

// UnmarshalShellCmdParts unmarshals shell command parts to the receiver
// QueryFull.  Note that no checks are performed as to whether HostsFilter,
// Time or Query are actually valid strings.
func (qf *QueryFull) UnmarshalShellCmdParts(parts []string) error {
	if len(parts) < numShellParts {
		return errors.Errorf(
			"not enough parts; should be at least %d, got %d", numShellParts, len(parts),
		)
	}

	if parts[0] != execName {
		return errors.Errorf("command should begin from %q, but it's %q", execName, parts[0])
	}

	parts = parts[1:]

	var hostsSet, timeSet, querySet bool

	for ; len(parts) >= 2; parts = parts[2:] {
		switch parts[0] {
		case "--hosts":
			qf.HostsFilter = parts[1]
			hostsSet = true
		case "--time":
			qf.Time = parts[1]
			timeSet = true
		case "--query":
			qf.Query = parts[1]
			querySet = true
		}
	}

	if !hostsSet {
		return errors.Errorf("--hosts is missing")
	}

	if !timeSet {
		return errors.Errorf("--time is missing")
	}

	if !querySet {
		return errors.Errorf("--query is missing")
	}

	return nil
}
