package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// NOTE: handleCmd is always called from the tview's event loop, so it's safe
// to use all UI primitives and nerdlogApp etc.
func (app *nerdlogApp) handleCmd(cmd string) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return
	}

	switch parts[0] {
	case "time":
		ftr, err := ParseFromToRange(strings.Join(parts[1:], " "))
		if err != nil {
			app.mainView.showMessagebox("err", "Error", err.Error(), nil)
			return
		}

		app.mainView.setTimeRange(ftr.From, ftr.To)
		app.mainView.doQuery()

	case "w", "write":
		//if len(parts) < 2 {
		//mainView.ShowMessagebox("err", "Error", ":write requires an argument: the filename to write", nil)
		//return
		//}

		//fname := parts[1]

		fname := "/tmp/last_nerdlog"
		if len(parts) >= 2 {
			fname = parts[1]
		}

		if app.lastLogResp == nil {
			app.mainView.showMessagebox("err", "Error", "No logs yet", nil)
			return
		}

		lfile, err := os.Create(fname)
		if err != nil {
			app.mainView.showMessagebox("err", "Error", fmt.Sprintf("Failed to open %s for writing: %s", fname, err), nil)
			return
		}

		for _, logMsg := range app.lastLogResp.Logs {
			fmt.Fprintf(lfile, "%s <ssh -t %s vim +%d %s>\n",
				logMsg.OrigLine,
				logMsg.Context["source"], logMsg.LogLinenumber, logMsg.LogFilename,
			)
		}

		lfile.Close()

		// TODO: make it less intrusive, just a message over command line like in vim.
		app.mainView.showMessagebox("err", "Fyi", fmt.Sprintf("Saved to %s", fname), nil)

	case "set":
		if len(parts) < 2 || len(parts[1]) == 0 {
			app.mainView.showMessagebox("err", "Error", "set requires an argument", nil)
			return
		}

		// TODO: implement in a generic way

		setParts := strings.Split(parts[1], "=")
		if len(setParts) == 2 {
			switch setParts[0] {
			case "numlines", "maxnumlines":
				val, err := strconv.Atoi(setParts[1])
				if err != nil {
					app.mainView.showMessagebox("err", "Error", "Can't parse "+setParts[1], nil)
					return
				}

				if val < 2 {
					app.mainView.showMessagebox("err", "Error", "numlines must be at least 2", nil)
					return
				}

				app.maxNumLines = val

			default:
				app.mainView.showMessagebox("err", "Error", "Unknown variable "+setParts[0], nil)
				return
			}
			return
		}

		if parts[1][len(parts[1])-1] == '?' {
			vn := parts[1][:len(parts[1])-1]
			switch vn {
			case "numlines", "maxnumlines":
				app.mainView.showMessagebox("err", "", "numlines is "+strconv.Itoa(app.maxNumLines), nil)

			default:
				app.mainView.showMessagebox("err", "Error", "Unknown variable "+vn, nil)
				return
			}
			return
		}

		app.mainView.showMessagebox("err", "Error", "Invalid set command"+string(parts[1][len(parts)-1]), nil)

	default:
		app.mainView.showMessagebox("err", "Error", fmt.Sprintf("unknown command %q", parts[0]), nil)
	}
}
