package core

import "fmt"

type HostsManager struct {
	has map[string]*HostAgent

	stateCh chan hostAgentsStateUpdate
}

type HostsManagerParams struct {
	ConfigHosts []ConfigHost
}

func NewHostsManager(params HostsManagerParams) *HostsManager {
	hm := &HostsManager{
		has: make(map[string]*HostAgent, len(params.ConfigHosts)),

		stateCh: make(chan hostAgentsStateUpdate, 32),
	}

	for _, hc := range params.ConfigHosts {
		ha := NewHostAgent(HostAgentParams{
			Config:  hc,
			StateCh: statesReceiver(hc.Name, hm.stateCh),
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
		case supd := <-hm.stateCh:
			delete(agentsByState[supd.oldState], supd.name)

			set, ok := agentsByState[supd.newState]
			if !ok {
				set = map[string]struct{}{}
				agentsByState[supd.newState] = set
			}

			set[supd.name] = struct{}{}

			fmt.Printf("Connected: %d/%d\n", len(agentsByState[HostAgentStateConnected]), len(hm.has))
		}
	}
}

type hostAgentsStateUpdate struct {
	name     string
	oldState HostAgentState
	newState HostAgentState
}

func statesReceiver(
	name string,
	ch chan<- hostAgentsStateUpdate,
) chan<- HostAgentStateUpdate {
	ret := make(chan HostAgentStateUpdate, 32)

	go func() {
		for {
			upd := <-ret
			ch <- hostAgentsStateUpdate{
				name:     name,
				oldState: upd.OldState,
				newState: upd.NewState,
			}
		}
	}()

	return ret
}
