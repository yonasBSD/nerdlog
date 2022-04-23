package main

import (
	"os"

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

	// lastLogResp contains the last response from HostsManager.
	lastLogResp *core.LogRespTotal
}

type nerdlogAppParams struct {
	initialQueryData QueryEditData
	connectRightAway bool
}

func newNerdlogApp(params nerdlogAppParams) *nerdlogApp {
	app := &nerdlogApp{
		params: params,

		tviewApp: tview.NewApplication(),

		maxNumLines: 250,
	}

	cmdCh := make(chan string, 8)

	app.mainView = NewMainView(&MainViewParams{
		App: app.tviewApp,
		OnLogQuery: func(params core.QueryLogsParams) {
			params.MaxNumLines = app.maxNumLines

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
	})

	// NOTE: initHostsManager has to be called _after_ app.mainView is initialized.
	app.initHostsManager("")

	if !params.connectRightAway {
		app.mainView.params.App.SetFocus(app.mainView.logsTable)
		app.mainView.queryEditView.Show(params.initialQueryData)
	} else {
		if err := app.mainView.applyQueryEditData(params.initialQueryData); err != nil {
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
			app.handleCmd(cmd)
		})
	}
}
