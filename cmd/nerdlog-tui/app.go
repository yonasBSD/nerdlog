package main

import (
	"os"

	"github.com/dimonomid/nerdlog/blhistory"
	"github.com/dimonomid/nerdlog/clhistory"
	"github.com/dimonomid/nerdlog/core"
	"github.com/juju/errors"
	"github.com/rivo/tview"
)

type nerdlogApp struct {
	params nerdlogAppParams

	tviewApp *tview.Application

	hm       *core.HostsManager
	mainView *MainView

	// maxNumLines is how many log lines the nerdlog_query.sh will return at
	// most. Initially it's set to 250.
	maxNumLines int

	// cmdLineHistory is the command line history
	cmdLineHistory *clhistory.CLHistory

	// queryBLHistory is the history of queries, as shell strings like this:
	// - nerdlog --hosts 'my-host-*' --time -10h --query '/series_ids_string=.*\|1\|/'
	// - nerdlog --hosts 'my-host-*' --time -2h --query '/ping/'
	queryBLHistory *blhistory.BLHistory
	// queryCLHistory is tracking the same data as queryBLHistory (queries like
	// nerdlog --hosts .....), but it's command-line-like, and it can be
	// navigated on the query edit form.
	queryCLHistory *clhistory.CLHistory

	lastQueryFull QueryFull

	// lastLogResp contains the last response from HostsManager.
	lastLogResp *core.LogRespTotal
}

type nerdlogAppParams struct {
	initialQueryData QueryFull
	connectRightAway bool
	enableClipboard  bool
}

func newNerdlogApp(params nerdlogAppParams) *nerdlogApp {
	app := &nerdlogApp{
		params: params,

		tviewApp: tview.NewApplication(),

		maxNumLines: 250,

		cmdLineHistory: clhistory.New(clhistory.CLHistoryParams{
			Filename: "/tmp/herdlog_history", // TODO: store it in home directory
		}),
		queryBLHistory: blhistory.New(),
		queryCLHistory: clhistory.New(clhistory.CLHistoryParams{
			Filename: "/tmp/herdlog_query_history", // TODO: store it in home directory
		}),
	}

	cmdCh := make(chan string, 8)

	app.mainView = NewMainView(&MainViewParams{
		App: app.tviewApp,
		OnLogQuery: func(params core.QueryLogsParams) {
			params.MaxNumLines = app.maxNumLines

			// Get the current QueryFull and marshal it to a shell command.
			qf := app.mainView.getQueryFull()
			qfStr := qf.MarshalShellCmd()

			// Add this query shell command to the commandline-like history.
			app.queryCLHistory.Add(qfStr)

			// If needed, also add it to the browser-like history.
			if qf != app.lastQueryFull {
				app.lastQueryFull = qf
				if !params.DontAddHistoryItem {
					app.queryBLHistory.Add(qfStr)
				}
			}

			app.hm.QueryLogs(params)
		},
		OnHostsFilterChange: func(hostsFilter string) error {
			err := app.hm.SetHostsFilter(hostsFilter)
			if err != nil {
				return errors.Trace(err)
			}

			return nil
		},
		OnCmd: func(cmd string) {
			cmdCh <- cmd
		},

		CmdHistory:   app.cmdLineHistory,
		QueryHistory: app.queryCLHistory,
	})

	// NOTE: initHostsManager has to be called _after_ app.mainView is initialized.
	app.initHostsManager("")

	if !params.connectRightAway {
		app.mainView.params.App.SetFocus(app.mainView.logsTable)
		app.mainView.queryEditView.Show(params.initialQueryData)
	} else {
		if err := app.mainView.applyQueryEditData(params.initialQueryData, doQueryParams{}); err != nil {
			panic(err.Error())
		}
	}

	go app.handleCmdLine(cmdCh)

	return app
}

func (app *nerdlogApp) runTViewApp() error {
	return app.tviewApp.SetRoot(app.mainView.GetUIPrimitive(), true).Run()
}

// NOTE: initHostsManager has to be called _after_ app.mainView is initialized.
func (app *nerdlogApp) initHostsManager(initialHostsFilter string) {
	updatesCh := make(chan core.HostsManagerUpdate, 128)
	go func() {
		for {
			upd := <-updatesCh

			app.tviewApp.QueueUpdateDraw(func() {
				switch {
				case upd.State != nil:
					app.mainView.applyHMState(upd.State)

				case upd.LogResp != nil:
					if len(upd.LogResp.Errs) > 0 {
						// TODO: include other errors too, not only the first one
						app.mainView.showMessagebox("err", "Log query error", upd.LogResp.Errs[0].Error(), nil)
						return
					}

					app.mainView.applyLogs(upd.LogResp)
					app.lastLogResp = upd.LogResp

				default:
					panic("empty hosts manager update")
				}
			})
		}
	}()

	envUser := os.Getenv("USER")

	app.hm = core.NewHostsManager(core.HostsManagerParams{
		ConfigHosts:        makeConfigHosts(),
		InitialHostsFilter: initialHostsFilter,

		ClientID: envUser,

		UpdatesCh: updatesCh,
	})
}

func (app *nerdlogApp) handleCmdLine(cmdCh <-chan string) {
	for {
		cmd := <-cmdCh
		app.tviewApp.QueueUpdateDraw(func() {
			app.cmdLineHistory.Add(cmd)
			app.handleCmd(cmd)
		})
	}
}

// printError lets user know that there is an error. For now it just uses the
// showMessagebox, but I don't like it since it's too intrusive; at some point
// I want to refactor it to print a simple error message over the command line,
// like in Vim: this way it won't be getting in the way.
func (app *nerdlogApp) printError(msg string) {
	app.mainView.showMessagebox("err", "Error", msg, nil)
}

// printMsg prints a FYI kind of message. Also see notes for printError.
func (app *nerdlogApp) printMsg(msg string) {
	app.mainView.showMessagebox("err", "Fyi", msg, nil)
}
