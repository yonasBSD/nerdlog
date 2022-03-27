package core

import (
	"fmt"
	"sort"

	"github.com/juju/errors"
)

type HostsManager struct {
	params HostsManagerParams

	has      map[string]*HostAgent
	haStates map[string]HostAgentState

	hostUpdatesCh chan *HostAgentUpdate
	reqCh         chan hostsManagerReq
	respCh        chan hostCmdRes

	curQueryLogsCtx *manQueryLogsCtx
}

type HostsManagerParams struct {
	ConfigHosts []ConfigHost

	UpdatesCh chan<- HostsManagerUpdate
}

func NewHostsManager(params HostsManagerParams) *HostsManager {
	hm := &HostsManager{
		params: params,

		has:      make(map[string]*HostAgent, len(params.ConfigHosts)),
		haStates: make(map[string]HostAgentState, len(params.ConfigHosts)),

		hostUpdatesCh: make(chan *HostAgentUpdate, 1024),
		reqCh:         make(chan hostsManagerReq, 8),
		respCh:        make(chan hostCmdRes),
	}

	for _, hc := range params.ConfigHosts {
		ha := NewHostAgent(HostAgentParams{
			Config:    hc,
			UpdatesCh: hm.hostUpdatesCh,
		})
		hm.has[hc.Name] = ha
		hm.haStates[hc.Name] = HostAgentStateDisconnected
	}

	go hm.run()

	return hm
}

func (hm *HostsManager) run() {
	agentsByState := map[HostAgentState]map[string]struct{}{}
	for name := range hm.has {
		agentsByState[HostAgentStateDisconnected] = map[string]struct{}{
			name: {},
		}
	}

	for {
		select {
		case upd := <-hm.hostUpdatesCh:
			if upd.State != nil {
				hm.haStates[upd.Name] = upd.State.NewState
				hm.sendStateUpdate()
			} else {
				panic("empty update " + upd.Name)
			}

		case req := <-hm.reqCh:
			switch {
			case req.queryLogs != nil:
				if hm.curQueryLogsCtx != nil {
					hm.sendLogRespUpdate(&LogResp{
						Errs: []error{errors.Errorf("busy with another query")},
					})
					continue
				}

				hm.curQueryLogsCtx = &manQueryLogsCtx{
					resps: make(map[string]*LogResp, len(hm.has)),
					errs:  map[string]error{},
				}

				// sendStateUpdate must be done after setting curQueryLogsCtx.
				hm.sendStateUpdate()

				for _, ha := range hm.has {
					ha.EnqueueCmd(hostCmd{
						respCh: hm.respCh,
						queryLogs: &hostCmdQueryLogs{
							from:  req.queryLogs.From,
							to:    req.queryLogs.To,
							query: req.queryLogs.Query,
						},
					})
				}

			case req.ping:
				for _, ha := range hm.has {
					ha.EnqueueCmd(hostCmd{
						ping: &hostCmdPing{},
					})
				}
			}

		case resp := <-hm.respCh:

			switch {
			case hm.curQueryLogsCtx != nil:
				if resp.err != nil {
					hm.curQueryLogsCtx.errs[resp.hostname] = resp.err
				}

				switch v := resp.resp.(type) {
				case *LogResp:
					hm.curQueryLogsCtx.resps[resp.hostname] = v

					// If we collected responses from all nodes, handle them.
					if len(hm.curQueryLogsCtx.resps) == len(hm.has) {
						hm.mergeLogRespsAndSend()

						hm.curQueryLogsCtx = nil

						// sendStateUpdate must be done after setting curQueryLogsCtx.
						hm.sendStateUpdate()
					}

					//if resp.hostname == "my-host-01" {
					//fmt.Printf("HEY got resp %+v\n", v.MinuteStats)
					//}

				default:
					panic(fmt.Sprintf("unexpected resp type %T", v))
				}

			default:
				// TODO: proper update
				fmt.Println("dropping update")
			}

		}
	}
}

type hostsManagerReq struct {
	// Exactly one field must be non-nil

	queryLogs *QueryLogsParams
	ping      bool
}

func (hm *HostsManager) QueryLogs(params QueryLogsParams) {
	hm.reqCh <- hostsManagerReq{
		queryLogs: &params,
	}
}

func (hm *HostsManager) Ping() {
	hm.reqCh <- hostsManagerReq{
		ping: true,
	}
}

type manQueryLogsCtx struct {
	// resps is a map from host name to its response. Once all responses have
	// been collected, we'll start merging them together.
	resps map[string]*LogResp
	errs  map[string]error
}

type HostsManagerUpdate struct {
	// Exactly one of the fields below must be non-nil

	State   *HostsManagerState
	LogResp *LogResp
}

type HostsManagerState struct {
	NumHosts int

	HostsByState map[HostAgentState]map[string]struct{}

	// Busy is true when a query is in progress
	Busy bool
}

func (hm *HostsManager) sendStateUpdate() {
	hostsByState := map[HostAgentState]map[string]struct{}{}

	for name, state := range hm.haStates {
		set, ok := hostsByState[state]
		if !ok {
			set = map[string]struct{}{}
			hostsByState[state] = set
		}

		set[name] = struct{}{}
	}

	hm.params.UpdatesCh <- HostsManagerUpdate{
		State: &HostsManagerState{
			NumHosts:     len(hm.has),
			HostsByState: hostsByState,
			Busy:         hm.curQueryLogsCtx != nil,
		},
	}
}

func (hm *HostsManager) sendLogRespUpdate(resp *LogResp) {
	hm.params.UpdatesCh <- HostsManagerUpdate{
		LogResp: resp,
	}
}

func (hm *HostsManager) mergeLogRespsAndSend() {
	resps := hm.curQueryLogsCtx.resps
	errs := hm.curQueryLogsCtx.errs

	if len(errs) != 0 {
		errs2 := make([]error, 0, len(errs))
		for hostname, err := range errs {
			errs2 = append(errs2, errors.Annotatef(err, "%s", hostname))
		}

		sort.Slice(errs2, func(i, j int) bool {
			return errs2[i].Error() < errs2[j].Error()
		})

		hm.sendLogRespUpdate(&LogResp{
			Errs: errs2,
		})
	}

	ret := &LogResp{
		MinuteStats: make(map[int64]MinuteStatsItem),
	}

	for _, resp := range resps {
		for k, v := range resp.MinuteStats {
			ret.MinuteStats[k] = MinuteStatsItem{
				NumMsgs: ret.MinuteStats[k].NumMsgs + v.NumMsgs,
			}
		}

		for _, msg := range resp.Logs {
			ret.Logs = append(ret.Logs, msg)
		}
	}

	sort.SliceStable(ret.Logs, func(i, j int) bool {
		return ret.Logs[i].Time.Before(ret.Logs[j].Time)
	})

	hm.sendLogRespUpdate(ret)
}
