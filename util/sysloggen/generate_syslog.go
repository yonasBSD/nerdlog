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

func generateSyslogEntry(ts time.Time, layout string, parts SyslogParts) string {
	timestamp := ts.Format(layout)
	hostname := "myhost"

	if parts.Tag == "" {
		parts.Tag = randomElement(facilities)
	}

	if parts.Severity == "" {
		parts.Severity = randomElement(severities)
	}

	if parts.Pid == 0 {
		parts.Pid = rand.Intn(9000) + 100
	}

	if parts.Message == "" {
		parts.Message = randomElement(messages)
	}

	return fmt.Sprintf(
		"%s %s %s[%d]: <%s> %s",
		timestamp, hostname, parts.Tag, parts.Pid, parts.Severity, parts.Message,
	)
}

func randomStep(params *DelayCfg, lastDelay, curDelayDelta time.Duration) time.Duration {
	min := -100 * int64(time.Millisecond)
	max := 100 * int64(time.Millisecond)
	curDelayDelta += time.Duration(rand.Int63n(max-min) + min)
	//curDelayDelta += -time.Duration(rand.Int63n(max))
	//_ = min

	ret := lastDelay + curDelayDelta
	if ret < time.Duration(params.MinDelayMS)*time.Millisecond {
		ret = time.Duration(params.MinDelayMS) * time.Millisecond
	}
	if ret > time.Duration(params.MaxDelayMS)*time.Millisecond {
		//ret = time.Duration(params.MaxDelayMS) * time.Millisecond
	}

	return ret

	//min := int64(params.MinDelayMS) * int64(time.Millisecond)
	//max := int64(params.MaxDelayMS) * int64(time.Millisecond)
	//return time.Duration(rand.Int63n(max-min) + min)
}

type Params struct {
	// If TimeLayout is empty, "Jan _2 15:04:05" will be used.
	TimeLayout string

	StartTime     time.Time
	SecondLogTime time.Time

	// LogBasename is a name like "mylog", can be "/path/to/mylog" as well.
	// The first (older) log will have the ".1" appended to it.
	LogBasename string

	// NumLogs specifies the max amount of logs to generate. If 0, we never stop
	// generating logs, and once caught up with the current time (time.Now()), it
	// switches to the real-time mode and continues there forever, waiting for
	// appropriate durations before printing every line.
	NumLogs int

	MinDelayMS int
	MaxDelayMS int

	RandomSeed int

	// SkipIfPrevLogSizeIs and SkipIfLastLogSizeIs are an optimization:
	// if the log files already exist and are of these exact sizes, don't
	// generate anything.
	SkipIfPrevLogSizeIs int64
	SkipIfLastLogSizeIs int64

	Spikes []Spike
}

type DelayCfg struct {
	MinDelayMS int
	MaxDelayMS int
}

type SpikePhase struct {
	// If EndTime is zero, the phase will never end
	EndTime time.Time

	MinDelayMS int
	MaxDelayMS int

	Trend func(phasePercentage float64, minDelayMS, maxDelayMS int) DelayCfg
}

type SyslogParts struct {
	Tag      string
	Severity string
	Pid      int
	Message  string
}

type Spike struct {
	StartTime time.Time
	// All of SyslogParts fields are optional; what is not specified, will be random
	SyslogParts SyslogParts
	Phases      []SpikePhase
}

type streamCtx struct {
	spikeCfg Spike

	lastDelay     time.Duration
	curDelayDelta time.Duration

	nextMsgTime time.Time
}

func GenerateSyslog(params Params) error {
	if params.TimeLayout == "" {
		params.TimeLayout = "Jan _2 15:04:05"
	}

	sCtxs := []*streamCtx{
		// Main stream
		&streamCtx{
			spikeCfg: Spike{
				Phases: []SpikePhase{
					SpikePhase{
						MinDelayMS: params.MinDelayMS,
						MaxDelayMS: params.MaxDelayMS,
					},
				},
			},
		},
	}

	for _, spike := range params.Spikes {
		sCtxs = append(sCtxs, &streamCtx{
			spikeCfg: spike,
		})
	}

	type nextDelayAndMsg struct {
		time        time.Time
		syslogParts SyslogParts
	}

	getNextDelayAndMsg := func(curTime time.Time) nextDelayAndMsg {
		// Generate nextMsgTime for all active spikes
		for _, sc := range sCtxs {
			if sc.nextMsgTime.IsZero() {
				if curTime.Before(sc.spikeCfg.StartTime) {
					continue
				}

				phaseStartTime := sc.spikeCfg.StartTime

				if len(sc.spikeCfg.Phases) == 0 {
					panic("no phases")
				}
				var d *DelayCfg
				for _, phase := range sc.spikeCfg.Phases {
					if phase.EndTime.IsZero() || curTime.Before(phase.EndTime) {
						// Found the phase
						percentage := 0.0
						if !phase.EndTime.IsZero() {
							totalDur := phase.EndTime.Sub(phaseStartTime)
							elapsedDur := curTime.Sub(phaseStartTime)
							percentage = float64(elapsedDur) / float64(totalDur) * 100.0
						}

						var curDelayCfg DelayCfg
						if phase.Trend != nil {
							curDelayCfg = phase.Trend(percentage, phase.MinDelayMS, phase.MaxDelayMS)
						} else {
							curDelayCfg = DelayCfg{
								MinDelayMS: phase.MinDelayMS,
								MaxDelayMS: phase.MaxDelayMS,
							}
						}

						d = &curDelayCfg
						break
					}

					phaseStartTime = phase.EndTime
				}

				if d == nil {
					// the spike is over (all phases of it are over)
					continue
				}

				if sc.lastDelay == 0 {
					sc.lastDelay = time.Duration(d.MinDelayMS + (d.MaxDelayMS-d.MinDelayMS)/2)
				}

				dur := randomStep(d, sc.lastDelay, sc.curDelayDelta)
				newDelayDelta := sc.lastDelay - dur

				sc.curDelayDelta = newDelayDelta
				sc.lastDelay = dur
				sc.nextMsgTime = curTime.Add(dur)
			}
		}

		var earliestTime *time.Time
		var syslogParts *SyslogParts
		// Find the earliest time
		for _, sc := range sCtxs {
			if sc.nextMsgTime.IsZero() {
				continue
			}

			if earliestTime == nil || sc.nextMsgTime.Before(*earliestTime) {
				tmpTime := sc.nextMsgTime
				tmpParts := sc.spikeCfg.SyslogParts

				earliestTime = &tmpTime
				syslogParts = &tmpParts

				// Reset it so we'll generate it again next time
				sc.nextMsgTime = time.Time{}
			}
		}

		if earliestTime == nil {
			panic("earliestTime must not be nil at this point, since we have the 'main' spike")
		}

		return nextDelayAndMsg{
			time:        *earliestTime,
			syslogParts: *syslogParts,
		}
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

	fmt.Printf("Creating first log file %s\n", prevlogFname)

	file, err := os.Create(prevlogFname)
	if err != nil {
		return errors.Annotatef(err, "creating first log file")
	}
	defer file.Close()

	numLogFile := 0
	numLogsWritten := 0

	isRealtime := false

	for {
		next := getNextDelayAndMsg(curTime)
		if curTime.After(next.time) {
			panic(fmt.Sprintf("curTime %v can't be after next.time %v", curTime, next.time))
		}

		step := next.time.Sub(curTime)
		curTime = curTime.Add(step)

		now := time.Now()
		if params.NumLogs > 0 && numLogsWritten >= params.NumLogs {
			break
		}

		// Real-time transition: we've caught up to current time
		if params.NumLogs == 0 && curTime.After(now) {
			if !isRealtime {
				fmt.Printf("Switching to realtime mode\n")
				isRealtime = true
			}

			time.Sleep(step)
			curTime = time.Now()
		}

		if !curTime.Before(params.SecondLogTime) && numLogFile == 0 {
			numLogFile += 1
			file.Close()

			fmt.Printf("Switching to the next log file %s\n", lastlogFname)

			var err error
			file, err = os.Create(lastlogFname)
			if err != nil {
				return errors.Annotatef(err, "creating second log file")
			}
			defer file.Close()
		}

		logEntry := generateSyslogEntry(curTime, params.TimeLayout, next.syslogParts)
		_, err := file.WriteString(logEntry + "\n")
		if err != nil {
			return errors.Annotatef(err, "writing to log file")
		}

		numLogsWritten++
	}

	return nil
}
