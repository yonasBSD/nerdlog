package main

import (
	"os"
	"path/filepath"

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

type cmdWithOpts struct {
	cmd  string
	opts CmdOpts
}

func newNerdlogApp(params nerdlogAppParams) (*nerdlogApp, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, errors.Annotatef(err, "getting home dir")
	}

	cmdLineHistory, err := clhistory.New(clhistory.CLHistoryParams{
		Filename: filepath.Join(homeDir, ".nerdlog_history"),
	})
	if err != nil {
		return nil, errors.Annotatef(err, "initializing cmdline history")
	}

	queryCLHistory, err := clhistory.New(clhistory.CLHistoryParams{
		Filename: filepath.Join(homeDir, ".nerdlog_query_history"),
	})
	if err != nil {
		return nil, errors.Annotatef(err, "initializing query history")
	}

	app := &nerdlogApp{
		params: params,

		tviewApp: tview.NewApplication(),

		maxNumLines: 250,

		cmdLineHistory: cmdLineHistory,
		queryBLHistory: blhistory.New(),
		queryCLHistory: queryCLHistory,
	}

	cmdCh := make(chan cmdWithOpts, 8)

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
		OnCmd: func(cmd string, opts CmdOpts) {
			cmdCh <- cmdWithOpts{
				cmd:  cmd,
				opts: opts,
			}
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

	return app, nil
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

func (app *nerdlogApp) handleCmdLine(cmdCh <-chan cmdWithOpts) {
	for {
		cwo := <-cmdCh
		app.tviewApp.QueueUpdateDraw(func() {
			if !cwo.opts.Internal {
				app.cmdLineHistory.Add(cwo.cmd)
			}
			app.handleCmd(cwo.cmd)
		})
	}
}

// printError lets user know that there is an error by printing a simple error
// message over the command line, sort of like in Vim.
// Note that if command line is focused atm, the message will not be printed
// and it's a no-op.
func (app *nerdlogApp) printError(msg string) {
	app.mainView.printMsg(msg, nlMsgLevelErr)
}

// printMsg prints a FYI kind of message. Also see notes for printError.
func (app *nerdlogApp) printMsg(msg string) {
	app.mainView.printMsg(msg, nlMsgLevelInfo)
}
