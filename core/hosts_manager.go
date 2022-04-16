package core

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/juju/errors"
)

const (
	// maxNumLines is how many log lines the nerdlog_query.sh will return at
	// most.
	// TODO: make it more configurable, perhaps
	maxNumLines = 250
)

type HostsManager struct {
	params HostsManagerParams

	hcs map[string]ConfigHost

	hostsFilter     string
	matchingHANames map[string]struct{}

	has      map[string]*HostAgent
	haStates map[string]HostAgentState

	hostsByState    map[HostAgentState]map[string]struct{}
	numNotConnected int
	numUnused       int

	hostUpdatesCh chan *HostAgentUpdate
	reqCh         chan hostsManagerReq
	respCh        chan hostCmdRes

	curQueryLogsCtx *manQueryLogsCtx

	curLogs manLogsCtx
}

type HostsManagerParams struct {
	ConfigHosts        []ConfigHost
	InitialHostsFilter string

	UpdatesCh chan<- HostsManagerUpdate
}

func NewHostsManager(params HostsManagerParams) *HostsManager {
	hm := &HostsManager{
		params: params,
		hcs:    make(map[string]ConfigHost, len(params.ConfigHosts)),

		hostsFilter: params.InitialHostsFilter,

		has:      map[string]*HostAgent{},
		haStates: map[string]HostAgentState{},

		hostUpdatesCh: make(chan *HostAgentUpdate, 1024),
		reqCh:         make(chan hostsManagerReq, 8),
		respCh:        make(chan hostCmdRes),
	}

	// Populate hm.hcs
	for _, hc := range params.ConfigHosts {
		hm.hcs[hc.Name] = hc
	}

	hm.filterHANames()
	hm.updateHAs()
	hm.updateHostsByState()
	hm.sendStateUpdate()

	go hm.run()

	return hm
}

func (hm *HostsManager) filterHANames() {
	hm.matchingHANames = map[string]struct{}{}

	for _, hc := range hm.params.ConfigHosts {
		// TODO: proper filtering
		if hm.hostsFilter == "" {
			// pass
		} else if !strings.Contains(hc.Name, hm.hostsFilter) {
			continue
		}

		hm.matchingHANames[hc.Name] = struct{}{}
	}
}

func (hm *HostsManager) updateHAs() {
	// Close unused host agents
	for name, oldHA := range hm.has {
		if _, ok := hm.matchingHANames[name]; ok {
			// The host is still used
			continue
		}

		// We used to use this host, but now it's filtered out, so close it
		oldHA.Close()
		delete(hm.has, name)
		delete(hm.haStates, name)
	}

	// Create new host agents
	for name := range hm.matchingHANames {
		if _, ok := hm.has[name]; ok {
			// This host agent already exists
			continue
		}

		// We need to create a new host agent
		hc := hm.hcs[name]
		ha := NewHostAgent(HostAgentParams{
			Config:    hc,
			UpdatesCh: hm.hostUpdatesCh,
		})
		hm.has[hc.Name] = ha
		hm.haStates[hc.Name] = HostAgentStateDisconnected
	}
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
				// If we don't have state for this agent, it means it's unused and the
				// state is probably about being torn down, so ignore the update.
				if _, ok := hm.haStates[upd.Name]; !ok {
					continue
				}

				hm.haStates[upd.Name] = upd.State.NewState

				hm.updateHostsByState()
				hm.sendStateUpdate()
			} else if upd.TornDown {
				// do we need this event at all?
			}

		case req := <-hm.reqCh:
			switch {
			case req.queryLogs != nil:
				if hm.numNotConnected > 0 {
					hm.sendLogRespUpdate(&LogRespTotal{
						Errs: []error{errors.Errorf("not connected to all hosts yet")},
					})
					continue
				}

				if hm.curQueryLogsCtx != nil {
					hm.sendLogRespUpdate(&LogRespTotal{
						Errs: []error{errors.Errorf("busy with another query")},
					})
					continue
				}

				hm.curQueryLogsCtx = &manQueryLogsCtx{
					loadEarlier: req.queryLogs.LoadEarlier,
					resps:       make(map[string]*LogResp, len(hm.has)),
					errs:        map[string]error{},
				}

				// sendStateUpdate must be done after setting curQueryLogsCtx.
				hm.sendStateUpdate()

				for hostName, ha := range hm.has {
					var linesUntil int
					if req.queryLogs.LoadEarlier {
						// TODO: right now, this loadEarlier case isn't optimized at all:
						// we again query the whole timerange, and every node goes through
						// all same lines and builds all the same mstats again (which we
						// then ignore). We can optimize it; however honestly the actual
						// performance, as per my experiments, isn't going to be
						// SPECTACULARLY better. Just kinda marginally better (try loading
						// older logs with time period 5h or 1m: the 1m is somewhat faster,
						// but not super fast. That's the difference we're talking about)
						//
						// Anyway, the way to optimize it is as follows: we already have
						// mstats, so we know what kind of timeframe we should query to get
						// the next maxNumLines messages. So we should query only this time
						// range, and we should avoid building any mstats. This way, no
						// matter how large the current time period is, loading more
						// messages will be as fast as possible.

						// Set linesUntil
						if nodeCtx, ok := hm.curLogs.perNode[hostName]; ok {
							if len(nodeCtx.logs) > 0 {
								linesUntil = nodeCtx.logs[0].CombinedLinenumber
							}
						}
					}

					ha.EnqueueCmd(hostCmd{
						respCh: hm.respCh,
						queryLogs: &hostCmdQueryLogs{
							from:  req.queryLogs.From,
							to:    req.queryLogs.To,
							query: req.queryLogs.Query,

							linesUntil: linesUntil,
						},
					})
				}

			case req.updHostsFilter != nil:
				r := req.updHostsFilter
				if hm.numNotConnected > 0 {
					r.resCh <- errors.Errorf("not connected to all hosts yet")
					continue
				}

				if hm.curQueryLogsCtx != nil {
					r.resCh <- errors.Errorf("busy with another query")
					continue
				}

				hm.hostsFilter = r.filter
				hm.filterHANames()
				hm.updateHAs()
				hm.updateHostsByState()
				hm.sendStateUpdate()

				r.resCh <- nil

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

	queryLogs      *QueryLogsParams
	updHostsFilter *hostsManagerReqUpdHostsFilter
	ping           bool
}

type hostsManagerReqUpdHostsFilter struct {
	filter string
	resCh  chan<- error
}

func (hm *HostsManager) QueryLogs(params QueryLogsParams) {
	hm.reqCh <- hostsManagerReq{
		queryLogs: &params,
	}
}

func (hm *HostsManager) SetHostsFilter(filter string) error {
	resCh := make(chan error, 1)

	hm.reqCh <- hostsManagerReq{
		updHostsFilter: &hostsManagerReqUpdHostsFilter{
			filter: filter,
			resCh:  resCh,
		},
	}

	return <-resCh
}

func (hm *HostsManager) Ping() {
	hm.reqCh <- hostsManagerReq{
		ping: true,
	}
}

type manQueryLogsCtx struct {
	// If loadEarlier is true, it means we're only loading the logs _before_ the ones
	// we already had.
	loadEarlier bool

	// resps is a map from host name to its response. Once all responses have
	// been collected, we'll start merging them together.
	resps map[string]*LogResp
	errs  map[string]error
}

type manLogsCtx struct {
	minuteStats  map[int64]MinuteStatsItem
	numMsgsTotal int

	perNode map[string]*manLogsNodeCtx
}

type manLogsNodeCtx struct {
	logs          []LogMsg
	isMaxNumLines bool
}

type HostsManagerUpdate struct {
	// Exactly one of the fields below must be non-nil

	State   *HostsManagerState
	LogResp *LogRespTotal
}

type HostsManagerState struct {
	NumHosts int

	HostsByState map[HostAgentState]map[string]struct{}

	// NumUnused is the number of hosts available in the config but filtered out.
	NumUnused int

	// Connected is true when all hosts are connected.
	Connected bool
	// Busy is true when a query is in progress.
	Busy bool
}

func (hm *HostsManager) updateHostsByState() {
	hm.numNotConnected = 0
	hm.hostsByState = map[HostAgentState]map[string]struct{}{}

	for name, state := range hm.haStates {
		set, ok := hm.hostsByState[state]
		if !ok {
			set = map[string]struct{}{}
			hm.hostsByState[state] = set
		}

		set[name] = struct{}{}

		if !isStateConnected(state) {
			hm.numNotConnected++
		}
	}

	hm.numUnused = len(hm.params.ConfigHosts) - len(hm.has)
}

func (hm *HostsManager) sendStateUpdate() {
	hm.params.UpdatesCh <- HostsManagerUpdate{
		State: &HostsManagerState{
			NumHosts:     len(hm.has),
			HostsByState: hm.hostsByState,
			Connected:    hm.numNotConnected == 0,
			NumUnused:    hm.numUnused,
			Busy:         hm.curQueryLogsCtx != nil,
		},
	}
}

func (hm *HostsManager) sendLogRespUpdate(resp *LogRespTotal) {
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

		hm.sendLogRespUpdate(&LogRespTotal{
			Errs: errs2,
		})

		return
	}

	// If we're not adding to already existing logs, reset w/e we've had already,
	// and calculate minuteStats from the resps.
	if !hm.curQueryLogsCtx.loadEarlier {
		hm.curLogs = manLogsCtx{
			minuteStats: map[int64]MinuteStatsItem{},
			perNode:     map[string]*manLogsNodeCtx{},
		}

		for nodeName, resp := range resps {
			for k, v := range resp.MinuteStats {
				hm.curLogs.minuteStats[k] = MinuteStatsItem{
					NumMsgs: hm.curLogs.minuteStats[k].NumMsgs + v.NumMsgs,
				}

				hm.curLogs.numMsgsTotal += v.NumMsgs
			}

			hm.curLogs.perNode[nodeName] = &manLogsNodeCtx{
				logs:          resp.Logs,
				isMaxNumLines: len(resp.Logs) == maxNumLines,
			}
		}
	} else {
		// Add to existing logs
		for nodeName, resp := range resps {
			pn := hm.curLogs.perNode[nodeName]
			pn.logs = append(resp.Logs, pn.logs...)
			pn.isMaxNumLines = len(resp.Logs) == maxNumLines
		}
	}

	ret := &LogRespTotal{
		MinuteStats:   hm.curLogs.minuteStats,
		NumMsgsTotal:  hm.curLogs.numMsgsTotal,
		LoadedEarlier: hm.curQueryLogsCtx.loadEarlier,
	}

	var logsCoveredSince time.Time

	for _, pn := range hm.curLogs.perNode {
		ret.Logs = append(ret.Logs, pn.logs...)

		// If the timespan covered by logs from this host is shorter than what
		// we've seen before, remember it.
		if pn.isMaxNumLines && logsCoveredSince.Before(pn.logs[0].Time) {
			logsCoveredSince = pn.logs[0].Time
		}
	}

	sort.SliceStable(ret.Logs, func(i, j int) bool {
		return ret.Logs[i].Time.Before(ret.Logs[j].Time)
	})

	// Cut all potentially incomplete logs, only leave timespan that we're sure
	// we have covered from all nodes
	coveredSinceIdx := sort.Search(len(ret.Logs), func(i int) bool {
		return !ret.Logs[i].Time.Before(logsCoveredSince)
	})
	ret.Logs = ret.Logs[coveredSinceIdx:]

	hm.sendLogRespUpdate(ret)
}
