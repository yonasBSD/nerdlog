package core

import "time"

type QueryLogsParams struct {
	From time.Time
	To   time.Time

	Query string

	// If LoadEarlier is true, it means we're only loading the logs _before_ the ones
	// we already had.
	LoadEarlier bool
}

// LogResp is a log response from a single host
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

// LogRespTotal is a log response from a HostsManager. It's merged from
// multiple LogResp's and it also contains some extra field(s), e.g. LoadedEarlier.
type LogRespTotal struct {
	// If LoadedEarlier is true, it means we've just loaded more logs instead of replacing
	// the logs (the Logs slice still contains everything though).
	LoadedEarlier bool

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

	// LogFilename and LogLinenumber are file ane line number in that file
	LogFilename   string
	LogLinenumber int

	// CombinedLinenumber is the line number in pseudo-file: all (actually just
	// two) log files concatenated. This is the linenumbers output by the
	// nerdlog_query.sh for every "msg:" line, and this is the linenumber
	// which should be used for --lines-until param.
	CombinedLinenumber int

	Msg     string
	Context map[string]string

	OrigLine string
}
