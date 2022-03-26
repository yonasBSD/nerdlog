package core

import "time"

type LogResp struct {
	// MinuteStats is a map from the unix timestamp (in seconds) to the stats for
	// the minute starting at this timestamp.
	MinuteStats map[int64]MinuteStatsItem

	Logs []LogMsg

	Errs []error
}

type MinuteStatsItem struct {
	NumMsgs int
}

type LogMsg struct {
	Time    time.Time
	Msg     string
	Context map[string]string
}
