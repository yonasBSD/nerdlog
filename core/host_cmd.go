package core

type hostCmd struct {
	// respCh must be either nil, or 1-buffered and it'll receive exactly one
	// message.
	respCh chan hostCmdRes

	// Exactly one of the fields below must be non-nil.

	bootstrap *hostCmdBootstrap
	ping      *hostCmdPing
}

type hostCmdRes struct {
	hostname string

	err  error
	resp interface{}
}

type hostCmdBootstrap struct{}

type hostCmdPing struct{}
