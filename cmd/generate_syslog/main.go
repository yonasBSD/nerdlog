package main

import (
	"fmt"
	"os"
	"time"

	"github.com/dimonomid/nerdlog/util/sysloggen"
	"github.com/juju/errors"
)

func main() {
	if err := main2(); err != nil {
		fmt.Println("error:", err.Error())
		os.Exit(1)
	}
}

func main2() error {
	t, err := time.Parse(time.RFC3339, "2025-03-09T06:00:00Z")
	if err != nil {
		return errors.Trace(err)
	}

	t2, err := time.Parse(time.RFC3339, "2025-03-10T06:00:00Z")
	if err != nil {
		return errors.Trace(err)
	}

	err = sysloggen.GenerateSyslog(sysloggen.Params{
		StartTime:     t,
		SecondLogTime: t2,

		LogBasename: "randomlog",

		NumLogs:    4000000,
		MinDelayMS: 0,
		MaxDelayMS: 80,

		RandomSeed: 123,
	})
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}
