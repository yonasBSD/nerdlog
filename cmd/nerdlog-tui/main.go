package main

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"
	"golang.design/x/clipboard"
)

// TODO: make multiple of them
const inputTimeLayout = "Jan02 15:04"
const inputTimeLayoutMMHH = "15:04"

var (
	flagTime  = pflag.StringP("time", "t", "", "Time range in the same format as accepted by the UI. Examples: '1h', 'Mar27 12:00'")
	flagHosts = pflag.StringP("hosts", "h", "", "Hosts to connect to, as comma-separated glob patterns, e.g. 'my-host-*,my-host-*'")
	flagQuery = pflag.StringP("query", "q", "", "Initial query to execute, using awk syntax")
)

func main() {
	pflag.Parse()

	initialTime := "-1h"
	initialHosts := "my-host-*"
	initialQuery := ""
	initialSelectQuery := DefaultSelectQuery
	connectRightAway := false

	if *flagTime != "" {
		initialTime = *flagTime
		connectRightAway = true
	}

	if *flagHosts != "" {
		initialHosts = *flagHosts
		connectRightAway = true
	}

	if *flagQuery != "" {
		initialQuery = *flagQuery
		connectRightAway = true
	}

	initialQueryData := QueryFull{
		Time:        initialTime,
		Query:       initialQuery,
		HostsFilter: initialHosts,
		SelectQuery: initialSelectQuery,
	}

	enableClipboard := true
	if err := clipboard.Init(); err != nil {
		enableClipboard = false
		fmt.Println("NOTE: X Clipboard is not available")
	}

	app, err := newNerdlogApp(nerdlogAppParams{
		initialQueryData: initialQueryData,
		connectRightAway: connectRightAway,
		enableClipboard:  enableClipboard,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	fmt.Println("Starting UI ...")
	if err := app.runTViewApp(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	// We end up here when the user quits the UI

	fmt.Println("")
	fmt.Println("Closing connections...")

	fmt.Println("Have a nice day.")
}
