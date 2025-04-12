package core

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/juju/errors"

	"github.com/dimonomid/nerdlog/log"
)

type HostsManager struct {
	params HostsManagerParams

	hostsStr          string
	parsedLogSubjects map[string]*ConfigLogSubject

	has      map[string]*HostAgent
	haStates map[string]HostAgentState
	// haConnDetails only contains items for hosts which are in the
	// HostAgentStateConnectinConnecting state.
	haConnDetails map[string]ConnDetails
	// haBusyStages only contains items for hosts which are in the
	// HostAgentStateConnectedBusy state.
	haBusyStages map[string]BusyStage
	// haPendingTeardown contains info about HoastAgent-s that are being torn down.
	haPendingTeardown map[string]int

	hostsByState    map[HostAgentState]map[string]struct{}
	numNotConnected int

	hostUpdatesCh chan *HostAgentUpdate
	reqCh         chan hostsManagerReq
	respCh        chan hostCmdRes

	// teardownReqCh is written to once when Close is called.
	teardownReqCh chan struct{}
	// tearingDown is true if the teardown is in progress (after Close is called).
	tearingDown bool
	// torndownCh is closed once the teardown is fully completed.
	// Wait waits for it.
	torndownCh chan struct{}

	curQueryLogsCtx *manQueryLogsCtx

	curLogs manLogsCtx
}

type HostsManagerParams struct {
	// TODO: use it
	PredefinedConfigHosts []ConfigHost

	Logger *log.Logger

	InitialHosts string

	// ClientID is just an arbitrary string (should be filename-friendly though)
	// which will be appended to the nerdlog_query.sh and its cache filenames.
	//
	// Needed to make sure that different clients won't get conflicts over those
	// files when using the tool concurrently on the same nodes.
	ClientID string

	UpdatesCh chan<- HostsManagerUpdate
}

func NewHostsManager(params HostsManagerParams) *HostsManager {
	params.Logger = params.Logger.WithNamespaceAppended("LSMan")

	hm := &HostsManager{
		params: params,

		has:               map[string]*HostAgent{},
		haStates:          map[string]HostAgentState{},
		haConnDetails:     map[string]ConnDetails{},
		haBusyStages:      map[string]BusyStage{},
		haPendingTeardown: map[string]int{},

		hostUpdatesCh: make(chan *HostAgentUpdate, 1024),
		reqCh:         make(chan hostsManagerReq, 8),
		respCh:        make(chan hostCmdRes),

		teardownReqCh: make(chan struct{}, 1),
		torndownCh:    make(chan struct{}, 1),
	}

	if err := hm.setHostsFilter(params.InitialHosts); err != nil {
		panic("setHostsFilter didn't like the initial filter: " + err.Error())
	}

	hm.updateHAs()
	hm.updateHostsByState()
	hm.sendStateUpdate()

	go hm.run()

	return hm
}

// TODO: rename to something like setHosts
func (hm *HostsManager) setHostsFilter(hostsStr string) error {
	parsedLogSubjects := map[string]*ConfigLogSubject{}

	// TODO: when json is supported, splitting by commas will need to be improved.
	parts := strings.Split(hostsStr, ",")
	for _, part := range parts {
		if part == "" {
			continue
		}

		cfs, err := parseConfigHost(part)
		if err != nil {
			return errors.Annotatef(err, "parsing %q", part)
		}

		for _, ch := range cfs {
			key := ch.Name

			if _, exists := parsedLogSubjects[key]; exists {
				return errors.Errorf("the host %s is present at least twice", key)
			}

			parsedLogSubjects[key] = ch
		}

		//matcher, err := glob.Compile(part)
		//if err != nil {
		//return errors.Annotatef(err, "pattern %q", part)
		//}

		//numMatchedPart := 0

		//for _, hc := range hm.params.ConfigHosts {
		//if !matcher.MatchString(hc.Name) {
		//continue
		//}

		//matchingHANames[hc.Name] = struct{}{}
		//numMatchedPart++
		//}

		//if numMatchedPart == 0 {
		//return errors.Errorf("%q didn't match anything", part)
		//}
	}

	// All went well, remember the filter
	hm.hostsStr = hostsStr
	hm.parsedLogSubjects = parsedLogSubjects

	return nil
}

func (hm *HostsManager) updateHAs() {
	// Close unused host agents
	for key, oldHA := range hm.has {
		if _, ok := hm.parsedLogSubjects[key]; ok {
			// The host is still used
			continue
		}

		// We used to use this host, but now it's filtered out, so close it
		hm.params.Logger.Verbose1f("Closing LSClient %s", key)
		delete(hm.has, key)
		delete(hm.haStates, key)

		keyNew := fmt.Sprintf("OLD_%s_%s", randomString(4), key)
		hm.haPendingTeardown[keyNew] += 1
		oldHA.Close(keyNew)
	}

	// Create new host agents
	for key, hc := range hm.parsedLogSubjects {
		if _, ok := hm.has[key]; ok {
			// This host agent already exists
			continue
		}

		// We need to create a new host agent
		ha := NewHostAgent(HostAgentParams{
			Config:    *hc,
			Logger:    hm.params.Logger,
			ClientID:  hm.params.ClientID, //fmt.Sprintf("%s-%d", hm.params.ClientID, rand.Int()),
			UpdatesCh: hm.hostUpdatesCh,
		})
		hm.has[key] = ha
		hm.haStates[key] = HostAgentStateDisconnected
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
				if _, ok := hm.haStates[upd.Name]; ok {
					hm.params.Logger.Verbose1f(
						"Got state update from %s: %s -> %s",
						upd.Name, upd.State.OldState, upd.State.NewState,
					)

					hm.haStates[upd.Name] = upd.State.NewState

					// Maintain hm.haConnDetails
					if upd.State.NewState == HostAgentStateConnectedIdle ||
						upd.State.NewState == HostAgentStateConnectedBusy {
						delete(hm.haConnDetails, upd.Name)
					}

					// Maintain hm.haBusyStages
					if upd.State.NewState != HostAgentStateConnectedBusy {
						delete(hm.haBusyStages, upd.Name)
					}
				} else if _, ok := hm.haPendingTeardown[upd.Name]; ok {
					hm.params.Logger.Verbose1f(
						"Got state update from tearing-down %s: %s -> %s",
						upd.Name, upd.State.OldState, upd.State.NewState,
					)
				} else {
					hm.params.Logger.Warnf(
						"Got state update from unknown %s: %s -> %s",
						upd.Name, upd.State.OldState, upd.State.NewState,
					)
				}

				hm.updateHostsByState()
				hm.sendStateUpdate()
			} else if upd.ConnDetails != nil {
				hm.params.Logger.Verbose1f("ConnDetails for %s: %+v", upd.Name, *upd.ConnDetails)
				hm.haConnDetails[upd.Name] = *upd.ConnDetails
				hm.sendStateUpdate()
			} else if upd.BusyStage != nil {
				hm.haBusyStages[upd.Name] = *upd.BusyStage
				hm.sendStateUpdate()
			} else if upd.TornDown {
				// One of our HostAgent-s has just shut down, account for it properly.
				hm.haPendingTeardown[upd.Name] -= 1

				// Sanity check.
				if hm.haPendingTeardown[upd.Name] < 0 {
					panic(fmt.Sprintf("got TornDown update and haPendingTeardown[%s] becomes %d", upd.Name, hm.haPendingTeardown[upd.Name]))
				}

				// Check how many HostAgent-s are still in the process of teardown
				numPending := 0
				for _, v := range hm.haPendingTeardown {
					numPending += v
				}

				if numPending != 0 {
					hm.params.Logger.Verbose1f(
						"HostAgent %s teardown is completed, %d more are still pending",
						upd.Name, numPending,
					)
				} else {
					hm.params.Logger.Verbose1f("HostAgent %s teardown is completed, no more pending teardowns", upd.Name)

					// If the whole HostsManager was shutting down, we're done now.
					if hm.tearingDown {
						hm.params.Logger.Infof("HostsManager teardown is completed")
						close(hm.torndownCh)
						return
					}
				}

				// do we need this event at all?
			}

		case req := <-hm.reqCh:
			switch {
			case req.queryLogs != nil:
				if len(hm.has) == 0 {
					hm.sendLogRespUpdate(&LogRespTotal{
						Errs: []error{errors.Errorf("no matching hosts to get logs from")},
					})
					continue
				}

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

				if req.queryLogs.MaxNumLines == 0 {
					panic("req.queryLogs.MaxNumLines is zero")
				}

				hm.curQueryLogsCtx = &manQueryLogsCtx{
					req:       req.queryLogs,
					startTime: time.Now(),
					resps:     make(map[string]*LogResp, len(hm.has)),
					errs:      map[string]error{},
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
							maxNumLines: req.queryLogs.MaxNumLines,

							from:  req.queryLogs.From,
							to:    req.queryLogs.To,
							query: req.queryLogs.Query,

							linesUntil: linesUntil,
						},
					})
				}

			case req.updHostsFilter != nil:
				r := req.updHostsFilter
				hm.params.Logger.Infof("Hosts manager: update hosts filter: %s", r.filter)

				if hm.curQueryLogsCtx != nil {
					r.resCh <- errors.Errorf("busy with another query")
					continue
				}

				if err := hm.setHostsFilter(r.filter); err != nil {
					r.resCh <- errors.Trace(err)
					continue
				}

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

			case req.reconnect:
				hm.params.Logger.Infof("Reconnect command")
				if hm.curQueryLogsCtx != nil {
					hm.params.Logger.Infof("Forgetting the in-progress query")
					hm.curQueryLogsCtx = nil
				}
				for _, ha := range hm.has {
					ha.Reconnect()
				}
			}

		case resp := <-hm.respCh:

			switch {
			case hm.curQueryLogsCtx != nil:
				if resp.err != nil {
					hm.params.Logger.Errorf("Got an error response from %v: %s", resp.hostname, resp.err)
					hm.curQueryLogsCtx.errs[resp.hostname] = resp.err
				}

				switch v := resp.resp.(type) {
				case *LogResp:
					hm.curQueryLogsCtx.resps[resp.hostname] = v

					// If we collected responses from all nodes, handle them.
					if len(hm.curQueryLogsCtx.resps) == len(hm.has) {
						hm.params.Logger.Verbose1f(
							"Got logs from %v, this was the last one, query is completed",
							resp.hostname,
						)

						hm.mergeLogRespsAndSend()

						hm.curQueryLogsCtx = nil

						// sendStateUpdate must be done after setting curQueryLogsCtx.
						hm.sendStateUpdate()
					} else {
						hm.params.Logger.Verbose1f(
							"Got logs from %v, %d more to go",
							resp.hostname,
							len(hm.has)-len(hm.curQueryLogsCtx.resps),
						)
					}

				default:
					panic(fmt.Sprintf("unexpected resp type %T", v))
				}

			default:
				hm.params.Logger.Errorf("Dropping update from %s on the floor", resp.hostname)
			}

		case <-hm.teardownReqCh:
			hm.params.Logger.Infof("HostsManager teardown is started")
			hm.tearingDown = true
			hm.setHostsFilter("")
			hm.updateHAs()
		}
	}
}

// Close initiates the shutdown. It doesn't wait for the shutdown to complete;
// use Wait for it.
func (hm *HostsManager) Close() {
	select {
	case hm.teardownReqCh <- struct{}{}:
	default:
	}
}

// Wait waits for the HostsManager to tear down. Typically used after calling Close().
func (hm *HostsManager) Wait() {
	<-hm.torndownCh
}

type hostsManagerReq struct {
	// Exactly one field must be non-nil

	queryLogs      *QueryLogsParams
	updHostsFilter *hostsManagerReqUpdHostsFilter
	ping           bool
	reconnect      bool
}

type hostsManagerReqUpdHostsFilter struct {
	filter string
	resCh  chan<- error
}

func (hm *HostsManager) QueryLogs(params QueryLogsParams) {
	hm.params.Logger.Verbose1f("QueryLogs: %+v", params)
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

func (hm *HostsManager) Reconnect() {
	hm.reqCh <- hostsManagerReq{
		reconnect: true,
	}
}

type manQueryLogsCtx struct {
	req *QueryLogsParams

	startTime time.Time

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

	// NumConnected is how many nodes are actually connected
	NumConnected int

	// NoMatchingHosts is true when there are no matching hosts.
	NoMatchingHosts bool

	// Connected is true when all matching hosts (which should be more than 0)
	// are connected.
	Connected bool

	// Busy is true when a query is in progress.
	Busy bool

	ConnDetailsByHost map[string]ConnDetails
	BusyStageByHost   map[string]BusyStage

	// TearingDown contains host names whic are in the process of teardown.
	TearingDown []string
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
}

func (hm *HostsManager) sendStateUpdate() {
	numConnected := 0
	for _, state := range hm.haStates {
		if isStateConnected(state) {
			numConnected++
		}
	}

	connDetailsCopy := make(map[string]ConnDetails, len(hm.haConnDetails))
	for k, v := range hm.haConnDetails {
		connDetailsCopy[k] = v
	}

	busyStagesCopy := make(map[string]BusyStage, len(hm.haBusyStages))
	for k, v := range hm.haBusyStages {
		busyStagesCopy[k] = v
	}

	tearingDown := make([]string, 0, len(hm.haPendingTeardown))
	for k, num := range hm.haPendingTeardown {
		for i := 0; i < num; i++ {
			tearingDown = append(tearingDown, k)
		}
	}
	sort.Strings(tearingDown)

	upd := HostsManagerUpdate{
		State: &HostsManagerState{
			NumHosts:          len(hm.has),
			HostsByState:      hm.hostsByState,
			NumConnected:      numConnected,
			NoMatchingHosts:   hm.numNotConnected == 0 && numConnected == 0,
			Connected:         hm.numNotConnected == 0 && numConnected > 0,
			Busy:              hm.curQueryLogsCtx != nil,
			ConnDetailsByHost: connDetailsCopy,
			BusyStageByHost:   busyStagesCopy,
			TearingDown:       tearingDown,
		},
	}

	hm.params.UpdatesCh <- upd
}

func (hm *HostsManager) sendLogRespUpdate(resp *LogRespTotal) {
	resp.QueryDur = time.Since(hm.curQueryLogsCtx.startTime)

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
	if !hm.curQueryLogsCtx.req.LoadEarlier {
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
				isMaxNumLines: len(resp.Logs) == hm.curQueryLogsCtx.req.MaxNumLines,
			}
		}
	} else {
		// Add to existing logs
		for nodeName, resp := range resps {
			pn := hm.curLogs.perNode[nodeName]
			pn.logs = append(resp.Logs, pn.logs...)
			pn.isMaxNumLines = len(resp.Logs) == hm.curQueryLogsCtx.req.MaxNumLines
		}
	}

	ret := &LogRespTotal{
		MinuteStats:   hm.curLogs.minuteStats,
		NumMsgsTotal:  hm.curLogs.numMsgsTotal,
		LoadedEarlier: hm.curQueryLogsCtx.req.LoadEarlier,
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
		if !ret.Logs[i].Time.Equal(ret.Logs[j].Time) {
			return ret.Logs[i].Time.Before(ret.Logs[j].Time)
		}

		// TODO: make it less hacky, store source somewhere outside of Context as well.
		return ret.Logs[i].Context["source"] < ret.Logs[j].Context["source"]
	})

	// Cut all potentially incomplete logs, only leave timespan that we're sure
	// we have covered from all nodes
	coveredSinceIdx := sort.Search(len(ret.Logs), func(i int) bool {
		return !ret.Logs[i].Time.Before(logsCoveredSince)
	})
	ret.Logs = ret.Logs[coveredSinceIdx:]

	hm.sendLogRespUpdate(ret)
}

func randomString(length int) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"

	rand.Seed(time.Now().UnixNano()) // Seed once per call
	prefix := make([]byte, length)
	for i := range prefix {
		prefix[i] = charset[rand.Intn(len(charset))]
	}
	return string(prefix)
}
