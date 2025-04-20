package core

import "time"

const (
	// MaxNumLinesDefault is a default for QueryLogsParams.MaxNumLines below.
	MaxNumLinesDefault = 250
)

type QueryLogsParams struct {
	// maxNumLines is how many log lines the nerdlog_agent.sh will return at
	// most.
	MaxNumLines int

	From time.Time
	To   time.Time

	Query string

	// If LoadEarlier is true, it means we're only loading the logs _before_ the ones
	// we already had.
	LoadEarlier bool

	// If DontAddHistoryItem is true, the browser-like history will not be
	// populated with a new item (it should be used exactly when we're navigating
	// this browser-like history back and forth)
	DontAddHistoryItem bool
}

// LogResp is a log response from a single logstream
type LogResp struct {
	// MinuteStats is a map from the unix timestamp (in seconds) to the stats for
	// the minute starting at this timestamp.
	MinuteStats map[int64]MinuteStatsItem

	Logs []LogMsg

	// NumMsgsTotal is the total number of messages in the time range (and
	// included in MinuteStats). This number is usually larger than len(Logs).
	NumMsgsTotal int
}

// LogRespTotal is a log response from a LStreamsManager. It's merged from
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

	// QueryDur shows how long the query took.
	QueryDur time.Duration
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
	// nerdlog_agent.sh for every "msg:" line, and this is the linenumber
	// which should be used for --lines-until param.
	CombinedLinenumber int

	Msg     string
	Context map[string]string
	Level   LogLevel

	OrigLine string
}

type LogLevel string

const LogLevelUnknown LogLevel = ""
const LogLevelDebug LogLevel = "debug"
const LogLevelInfo LogLevel = "info"
const LogLevelWarn LogLevel = "warn"
const LogLevelError LogLevel = "error"
