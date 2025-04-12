package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/juju/errors"
	"golang.design/x/clipboard"
)

// NOTE: handleCmd is always called from the tview's event loop, so it's safe
// to use all UI primitives and nerdlogApp etc.
func (app *nerdlogApp) handleCmd(cmd string) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return
	}

	switch parts[0] {
	case "h", "help":
		app.mainView.showMessagebox("err", "Fyi", "The only help for now is the README.md in the repo, so check it out", &MessageboxParams{
			Width: 49,
		})

	case "time":
		ftr, err := ParseFromToRange(app.options.GetTimezone(), strings.Join(parts[1:], " "))
		if err != nil {
			app.printError(err.Error())
			return
		}

		app.mainView.setTimeRange(ftr.From, ftr.To)
		app.mainView.doQuery(doQueryParams{})

	case "w", "write":
		//if len(parts) < 2 {
		//app.printError(":write requires an argument: the filename to write")
		//return
		//}

		//fname := parts[1]

		fname := "/tmp/last_nerdlog"
		if len(parts) >= 2 {
			fname = parts[1]
		}

		if app.lastLogResp == nil {
			app.printError("No logs yet")
			return
		}

		lfile, err := os.Create(fname)
		if err != nil {
			app.printError(fmt.Sprintf("Failed to open %s for writing: %s", fname, err))
			return
		}

		for _, logMsg := range app.lastLogResp.Logs {
			fmt.Fprintf(lfile, "%s <ssh -t %s vim +%d %s>\n",
				logMsg.OrigLine,
				logMsg.Context["source"], logMsg.LogLinenumber, logMsg.LogFilename,
			)
		}

		lfile.Close()

		app.printMsg(fmt.Sprintf("Saved to %s", fname))

	case "set":
		if len(parts) < 2 || len(parts[1]) == 0 {
			app.printError("set requires an argument")
			return
		}

		// TODO: implement in a generic way

		setParts := strings.SplitN(parts[1], "=", 2)
		if len(setParts) == 2 {
			optName := setParts[0]
			optValue := setParts[1]

			if opt := OptionMetaByName(optName); opt != nil {
				var setErr error
				app.options.Call(func(o *Options) {
					setErr = opt.Set(o, optValue)
				})

				if setErr != nil {
					app.printError(setErr.Error())
					return
				}

				return
			}

			app.printError("Unknown variable " + optName)
			return
		}

		if parts[1][len(parts[1])-1] == '?' {
			optName := parts[1][:len(parts[1])-1]

			if opt := OptionMetaByName(optName); opt != nil {
				var optValue string
				app.options.Call(func(o *Options) {
					optValue = opt.Get(o)
				})

				app.printMsg(fmt.Sprintf("%s is %s", optName, optValue))
				return
			}

			app.printError("Unknown variable " + optName)
			return
		}

		app.printError("Invalid set command")

	case "xc", "xclip":
		qf := app.mainView.getQueryFull()
		shellCmd := qf.MarshalShellCmd()
		if app.params.enableClipboard {
			clipboard.Write(clipboard.FmtText, []byte(shellCmd))
			app.printMsg("Copied to clipboard")
		} else {
			app.printMsg("Clipboard is not available, command: " + shellCmd)
		}

	case "nerdlog":
		// Mimic as if it was called from a shell

		if err := app.unmarshalAndApplyQuery(cmd, doQueryParams{}); err != nil {
			app.printError(err.Error())
			return
		}

	case "prev", "bac", "bck", "back":
		item := app.queryBLHistory.Prev()
		if item == nil {
			app.printError("No more history items")
			return
		}

		if err := app.unmarshalAndApplyQuery(item.Str, doQueryParams{
			dontAddHistoryItem: true,
		}); err != nil {
			// This shouldn't happen really provided a sane history.
			app.printError(err.Error())
			return
		}

		// TODO: print history item stats

	case "next", "fwd", "forward":
		item := app.queryBLHistory.Next()
		if item == nil {
			app.printError("No more history items")
			return
		}

		if err := app.unmarshalAndApplyQuery(item.Str, doQueryParams{
			dontAddHistoryItem: true,
		}); err != nil {
			// This shouldn't happen really provided a sane history.
			app.printError(err.Error())
			return
		}

		// TODO: print history item stats

	case "e", "edit":
		app.mainView.openQueryEditView()

	case "q", "quit":
		app.tviewApp.Stop()

	case "reconnect":
		// TODO: make it a mainView own function, and we'll also use it whenever
		// we trigger reconnection from the UI.
		app.mainView.doQueryParamsOnceConnected = &doQueryParams{}
		app.hm.Reconnect()

	case "cancel", "stop", "abort":
		app.mainView.doQueryParamsOnceConnected = &doQueryParams{}
		app.hm.AbortQuery()

	default:
		app.printError(fmt.Sprintf("unknown command %q", parts[0]))
	}
}

func (app *nerdlogApp) unmarshalAndApplyQuery(cmd string, dqp doQueryParams) error {
	var qf QueryFull
	if err := qf.UnmarshalShellCmd(cmd); err != nil {
		return errors.Annotatef(err, "parsing")
	}

	if err := app.mainView.applyQueryEditData(qf, dqp); err != nil {
		return errors.Annotatef(err, "applying")
	}

	return nil
}
