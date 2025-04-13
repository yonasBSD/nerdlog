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
	flagTime        = pflag.StringP("time", "t", "", "Time range in the same format as accepted by the UI. Examples: '1h', 'Mar27 12:00'")
	flagLStreams    = pflag.StringP("lstreams", "h", "", "Logstreams to connect to, as comma-separated glob patterns, e.g. 'foo-*,bar-*'")
	flagQuery       = pflag.StringP("query", "q", "", "Initial query to execute, using awk syntax")
	flagSelectQuery = pflag.StringP("selquery", "s", "", "SELECT-like query to specify which fields to show, like 'time STICKY, message, source, level_name AS level, *'")
)

func main() {
	pflag.Parse()

	initialTime := "-1h"
	initialLStreams := "localhost"
	initialQuery := ""
	initialSelectQuery := DefaultSelectQuery
	connectRightAway := false

	if *flagTime != "" {
		initialTime = *flagTime
		connectRightAway = true
	}

	if *flagLStreams != "" {
		initialLStreams = *flagLStreams
		connectRightAway = true
	}

	if *flagQuery != "" {
		initialQuery = *flagQuery
		connectRightAway = true
	}

	if *flagSelectQuery != "" {
		initialSelectQuery = SelectQuery(*flagSelectQuery)
		connectRightAway = true
	}

	initialQueryData := QueryFull{
		Time:        initialTime,
		Query:       initialQuery,
		LStreams:    initialLStreams,
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

	app.Close()
	app.Wait()

	fmt.Println("Have a nice day.")
}
