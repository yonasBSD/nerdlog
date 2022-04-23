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
			app.printError(err.Error())
			return
		}

		app.mainView.setTimeRange(ftr.From, ftr.To)
		app.mainView.doQuery()

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

		setParts := strings.Split(parts[1], "=")
		if len(setParts) == 2 {
			switch setParts[0] {
			case "numlines", "maxnumlines":
				val, err := strconv.Atoi(setParts[1])
				if err != nil {
					app.printError("Can't parse " + setParts[1])
					return
				}

				if val < 2 {
					app.printError("numlines must be at least 2")
					return
				}

				app.maxNumLines = val

			default:
				app.printError("Unknown variable " + setParts[0])
				return
			}
			return
		}

		if parts[1][len(parts[1])-1] == '?' {
			vn := parts[1][:len(parts[1])-1]
			switch vn {
			case "numlines", "maxnumlines":
				app.printError("numlines is " + strconv.Itoa(app.maxNumLines))

			default:
				app.printError("Unknown variable " + vn)
				return
			}
			return
		}

		app.printError("Invalid set command" + string(parts[1][len(parts)-1]))

	default:
		app.printError(fmt.Sprintf("unknown command %q", parts[0]))
	}
}
