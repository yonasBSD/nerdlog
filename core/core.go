package core

import "time"

type QueryLogsParams struct {
	From time.Time
	To   time.Time

	Query string
}

type LogResp struct {
	// MinuteStats is a map from the unix timestamp (in seconds) to the stats for
	// the minute starting at this timestamp.
	MinuteStats map[int64]MinuteStatsItem

	Logs []LogMsg

	// NumMsgsTotal is the total number of messages in the time range (and
	// included in MinuteStats). This number is usually larger than len(Logs).
	NumMsgsTotal int

	Errs []error
}

type MinuteStatsItem struct {
	NumMsgs int
}

type LogMsg struct {
	Time               time.Time
	DecreasedTimestamp bool

	LogFilename   string
	LogLinenumber int

	Msg     string
	Context map[string]string

	OrigLine string
}
