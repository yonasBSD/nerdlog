package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var logFile *os.File
var logFileMtx sync.Mutex

// Printf prints a formatted message to the log file ~/.nerdlog.log
func Printf(format string, a ...interface{}) {
	w := writer()

	fmt.Fprintf(w, "%s: ", time.Now().Format(time.RFC3339Nano))

	if !strings.HasSuffix(format, "\n") {
		format += "\n"
	}

	fmt.Fprintf(w, format, a...)
}

func writer() io.Writer {
	logFileMtx.Lock()
	defer logFileMtx.Unlock()

	if logFile == nil {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			panic(err.Error())
		}

		fname := filepath.Join(homeDir, ".nerdlog.log")

		logFile, err = os.OpenFile(fname, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			panic(err.Error())
		}
	}

	return logFile
}
