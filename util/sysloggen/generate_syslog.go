package sysloggen

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/juju/errors"
)

var facilities = []string{
	"kern", "user", "mail", "daemon", "auth", "syslog",
	"lpr", "news", "uucp", "cron", "authpriv", "ftp",
}

var severities = []string{
	"emerg", "alert", "crit", "err", "warning", "notice", "info", "debug",
}

var messages = []string{
	"Connection established",
	"User login successful",
	"Disk space low",
	"System rebooted",
	"Configuration updated",
	"Unauthorized access attempt",
	"Service started",
	"File not found",
	"Network interface down",
	"Memory usage high",
	"Process terminated",
	"CPU temperature critical",
	"New device connected",
	"Package installation completed",
	"Database query failed",
	"Service stopped",
	"Error reading file",
	"Permission denied",
	"Disk write error",
	"Failed login attempt",
	"Configuration load failed",
	"Authentication failure",
	"Server shutting down",
	"Session expired",
	"Process started",
	"Timeout occurred",
	"Network unreachable",
	"Port unreachable",
	"Insufficient privileges",
	"DNS resolution failed",
	"Database connection error",
	"Configuration reload successful",
	"File system full",
	"System performance degraded",
	"Kernel panic",
	"Backup completed",
	"Backup failed",
	"File copied successfully",
	"Process crashed",
	"Memory leak detected",
	"New update available",
	"Update failed",
	"Service restart requested",
	"Database schema updated",
	"Invalid input detected",
	"Request timed out",
	"Resource allocation failed",
	"Firewall rule added",
	"Firewall rule deleted",
	"User session started",
	"User session ended",
	"Disk format completed",
	"SSH connection established",
	"SSH connection closed",
	"Unexpected error occurred",
	"Request successfully processed",
	"Service restart completed",
	"Cache cleared",
	"Cache update completed",
	"Configuration applied successfully",
	"Service health check failed",
	"System running low on resources",
	"File system check completed",
	"Software version updated",
	"Memory usage normal",
	"Disk usage critical",
	"Network congestion detected",
	"Network speed reduced",
	"Service unavailable",
	"Hardware failure detected",
	"Service dependency failure",
	"System time updated",
	"Invalid credentials provided",
	"Login attempt locked out",
	"User account disabled",
	"User account enabled",
	"Service request queued",
	"Service request completed",
	"User permissions updated",
	"Certificate expiration warning",
	"Application crash reported",
	"Security patch applied",
	"Backup restoration completed",
	"Maintenance mode enabled",
	"Maintenance mode disabled",
	"Security alert raised",
	"Security breach detected",
	"Resource utilization warning",
	"High CPU usage detected",
	"High memory usage detected",
	"Disk error occurred",
	"Data corruption detected",
	"API request failed",
	"API response received",
	"File checksum mismatch",
	"Hardware upgrade completed",
	"System configuration backed up",
	"User session timed out",
	"System configuration restored",
	"Service initialization failed",
	"Server started successfully",
	"Server stopped unexpectedly",
	"Logging level changed",
	"Error handling request",
	"Out of memory error",
	"Service dependency initialized",
	"Scheduled task executed",
	"Scheduled task failed",
	"User authentication successful",
	"User authentication failed",
	"Session token expired",
	"File upload completed",
	"File download started",
	"File upload failed",
	"File download failed",
	"File transfer completed",
	"File transfer failed",
	"System clock synchronized",
	"System time drift detected",
	"Software upgrade completed",
	"System reboot required",
	"Disk space reclaimed",
	"IP address conflict detected",
	"Application configuration error",
	"System health check completed",
	"System health check failed",
	"Log file rotated",
	"Log file archived",
	"User password changed",
	"Invalid password attempt",
	"SMTP server connection error",
	"Database migration completed",
	"Database migration failed",
	"Network link restored",
	"Network interface reset",
}

func randomElement(list []string) string {
	return list[rand.Intn(len(list))]
}

func generateSyslogEntry(ts time.Time, layout string) string {
	timestamp := ts.Format(layout)
	hostname := "myhost"
	tag := randomElement(facilities)
	severity := randomElement(severities)
	pid := rand.Intn(9000) + 100
	message := randomElement(messages)

	return fmt.Sprintf("%s %s %s[%d]: <%s> %s", timestamp, hostname, tag, pid, severity, message)
}

func randomStep(params *Params) time.Duration {
	//if rand.Float64() < 0.2 {
	//return 1 * time.Millisecond
	//}

	min := int64(params.MinDelayMS) * int64(time.Millisecond)
	max := int64(params.MaxDelayMS) * int64(time.Millisecond)
	return time.Duration(rand.Int63n(max-min) + min)
}

type Params struct {
	// If TimeLayout is empty, "Jan _2 15:04:05" will be used.
	TimeLayout string

	StartTime     time.Time
	SecondLogTime time.Time

	// LogBasename is a name like "mylog", can be "/path/to/mylog" as well.
	// The first (older) log will have the ".1" appended to it.
	LogBasename string

	NumLogs int

	MinDelayMS int
	MaxDelayMS int

	RandomSeed int

	// SkipIfPrevLogSizeIs and SkipIfLastLogSizeIs are an optimization:
	// if the log files already exist and are of these exact sizes, don't
	// generate anything.
	SkipIfPrevLogSizeIs int64
	SkipIfLastLogSizeIs int64
}

func GenerateSyslog(params Params) error {
	if params.TimeLayout == "" {
		params.TimeLayout = "Jan _2 15:04:05"
	}

	rand.Seed(int64(params.RandomSeed))

	curTime := params.StartTime

	prevlogFname := params.LogBasename + ".1"
	lastlogFname := params.LogBasename

	if params.SkipIfPrevLogSizeIs != 0 && params.SkipIfLastLogSizeIs != 0 {
		var prevlogSize int64
		var lastlogSize int64

		prevlogStat, err := os.Stat(prevlogFname)
		if err == nil {
			prevlogSize = prevlogStat.Size()
		}

		lastlogStat, err := os.Stat(lastlogFname)
		if err == nil {
			lastlogSize = lastlogStat.Size()
		}

		if params.SkipIfPrevLogSizeIs == prevlogSize &&
			params.SkipIfLastLogSizeIs == lastlogSize {
			// Nothing to do
			//fmt.Println("Log files already exist and are of expected sizes, skipping generation")
			return nil
		}
	}

	fmt.Println("Generating log files...")

	file, err := os.Create(prevlogFname)
	if err != nil {
		return errors.Annotatef(err, "creating first log file")
	}
	defer file.Close()

	numLogFile := 0

	for i := 0; i < params.NumLogs; i++ {
		curTime = curTime.Add(randomStep(&params))
		if !curTime.Before(params.SecondLogTime) && numLogFile == 0 {
			numLogFile += 1
			file.Close()

			var err error
			file, err = os.Create(lastlogFname)
			if err != nil {
				return errors.Annotatef(err, "creating second log file")
			}
			defer file.Close()
		}

		logEntry := generateSyslogEntry(curTime, params.TimeLayout)
		_, err := file.WriteString(logEntry + "\n")
		if err != nil {
			return errors.Annotatef(err, "writing to log file")
		}
	}

	return nil
}
