package core

import "time"

type hostCmd struct {
	// respCh must be either nil, or 1-buffered and it'll receive exactly one
	// message.
	respCh chan hostCmdRes

	// Exactly one of the fields below must be non-nil.

	bootstrap *hostCmdBootstrap
	ping      *hostCmdPing
	queryLogs *hostCmdQueryLogs
}

type hostCmdCtx struct {
	cmd hostCmd

	idx int

	bootstrapCtx *hostCmdCtxBootstrap
	pingCtx      *hostCmdCtxPing
	queryLogsCtx *hostCmdCtxQueryLogs
}

type hostCmdRes struct {
	hostname string

	err  error
	resp interface{}
}

type hostCmdBootstrap struct{}

type hostCmdCtxBootstrap struct {
	receivedSuccess bool
	receivedFailure bool
}

type hostCmdPing struct{}

type hostCmdCtxPing struct {
}

type hostCmdQueryLogs struct {
	from time.Time
	to   time.Time

	query string
}

type hostCmdCtxQueryLogs struct {
	Resp *LogResp
}
