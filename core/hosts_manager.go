package core

import "fmt"

type HostsManager struct {
	has map[string]*HostAgent

	updatesCh chan *HostAgentUpdate
}

type HostsManagerParams struct {
	ConfigHosts []ConfigHost
}

func NewHostsManager(params HostsManagerParams) *HostsManager {
	hm := &HostsManager{
		has: make(map[string]*HostAgent, len(params.ConfigHosts)),

		updatesCh: make(chan *HostAgentUpdate, 1024),
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
		}
	}
}
