package core

import "time"

type lstreamCmd struct {
	// respCh must be either nil, or 1-buffered and it'll receive exactly one
	// message.
	respCh chan lstreamCmdRes

	// Exactly one of the fields below must be non-nil.

	bootstrap *lstreamCmdBootstrap
	ping      *lstreamCmdPing
	queryLogs *lstreamCmdQueryLogs
}

type lstreamCmdCtx struct {
	cmd lstreamCmd

	idx int

	bootstrapCtx *lstreamCmdCtxBootstrap
	pingCtx      *lstreamCmdCtxPing
	queryLogsCtx *lstreamCmdCtxQueryLogs
}

type lstreamCmdRes struct {
	hostname string

	err  error
	resp interface{}
}

type lstreamCmdBootstrap struct{}

type lstreamCmdCtxBootstrap struct {
	receivedSuccess bool
	receivedFailure bool
}

type lstreamCmdPing struct{}

type lstreamCmdCtxPing struct {
}

type lstreamCmdQueryLogs struct {
	maxNumLines int

	from time.Time
	to   time.Time

	query string

	// If linesUntil is not zero, it'll be passed to nerdlog_agent.sh as --lines-until.
	// Effectively, only logs BEFORE this log line (not including it) will be output.
	linesUntil int
}

type lstreamCmdCtxQueryLogs struct {
	Resp *LogResp

	logfiles []logfileWithStartingLinenumber
	lastTime time.Time
}

type logfileWithStartingLinenumber struct {
	filename       string
	fromLinenumber int
}
