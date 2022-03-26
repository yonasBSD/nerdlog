package core

import (
	"fmt"
	"time"
)

type HostsManager struct {
	has map[string]*HostAgent

	updatesCh chan *HostAgentUpdate
	reqCh     chan hostsManagerReq
	respCh    chan hostCmdRes
}

type HostsManagerParams struct {
	ConfigHosts []ConfigHost
}

func NewHostsManager(params HostsManagerParams) *HostsManager {
	hm := &HostsManager{
		has: make(map[string]*HostAgent, len(params.ConfigHosts)),

		updatesCh: make(chan *HostAgentUpdate, 1024),
		reqCh:     make(chan hostsManagerReq, 8),
		respCh:    make(chan hostCmdRes),
	}

	for _, hc := range params.ConfigHosts {
		ha := NewHostAgent(HostAgentParams{
			Config:    hc,
			UpdatesCh: hm.updatesCh,
		})
		hm.has[hc.Name] = ha
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

	//ticker := time.NewTicker(15 * time.Second)

	for {
		select {
		case upd := <-hm.updatesCh:
			if upd.State != nil {
				supd := upd.State
				delete(agentsByState[supd.OldState], upd.Name)

				set, ok := agentsByState[supd.NewState]
				if !ok {
					set = map[string]struct{}{}
					agentsByState[supd.NewState] = set
				}

				set[upd.Name] = struct{}{}

				fmt.Printf(
					"Connected: %d/%d (idle %d, busy %d)\n",
					len(agentsByState[HostAgentStateConnectedIdle])+len(agentsByState[HostAgentStateConnectedBusy]),
					len(hm.has),
					len(agentsByState[HostAgentStateConnectedIdle]),
					len(agentsByState[HostAgentStateConnectedBusy]),
				)
			} else {
				panic("empty update " + upd.Name)
			}

			/*
				case <-ticker.C:
					for _, ha := range hm.has {
						ha.EnqueueCmd(hostCmd{
							queryLogs: &hostCmdQueryLogs{
								From: time.Now().Add(-1 * time.Hour),
							},
						})
					}
			*/

		case req := <-hm.reqCh:
			switch {
			case req.queryLogs != nil:
				for _, ha := range hm.has {
					ha.EnqueueCmd(hostCmd{
						respCh: hm.respCh,
						queryLogs: &hostCmdQueryLogs{
							from: req.queryLogs.From,
							to:   req.queryLogs.To,
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
			switch v := resp.resp.(type) {
			case *LogResp:
				//if resp.hostname == "my-host-01" {
				fmt.Printf("HEY got resp %+v\n", v.MinuteStats)
				//}

			default:
				panic(fmt.Sprintf("unexpected resp type %T", v))
			}

		}
	}
}

type hostsManagerReq struct {
	// Exactly one field must be non-nil

	queryLogs *QueryLogsParams
	ping      bool
}

type QueryLogsParams struct {
	From time.Time
	To   time.Time
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
