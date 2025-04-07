package main

import (
	"time"
)

// getXMarksForTimeRange takes a time range which represent a timescale, and
// returns timestamps of a few marks that make sense to put on the timescale
// for the human to see; the number of marks is as close to maxNumMarks, but not
// larger than this.
//
// The returned marks are on the most round places: e.g. if there are multiple
// days, then at least some marks must be on the day boundary; the marks are
// usually divisible by 5, 10, 30, or 60 mins, etc.
func getXMarksForTimeRange(from, to time.Time, maxNumMarks int) []time.Time {
	if !from.Before(to) || maxNumMarks <= 0 {
		return nil
	}

	duration := to.Sub(from)
	step := chooseStep(duration, maxNumMarks)
	if step == 0 {
		return nil
	}

	// Align the first mark to the step boundary.
	start := from.Truncate(step)
	if start.Before(from) {
		start = start.Add(step)
	}

	var marks []time.Time
	for t := start; !t.After(to); t = t.Add(step) {
		marks = append(marks, t)
		if len(marks) >= maxNumMarks {
			break
		}
	}
	return marks
}

var snaps = []time.Duration{
	time.Minute * 1,
	time.Minute * 2,
	time.Minute * 5,
	time.Minute * 10,
	time.Minute * 15,
	time.Minute * 20,
	time.Minute * 30,
	time.Hour * 1,
	time.Hour * 2,
	time.Hour * 3,
	time.Hour * 6,
	time.Hour * 12,
	time.Hour * 24,
	time.Hour * 24 * 2,
	time.Hour * 24 * 7,
	time.Hour * 24 * 30,
	time.Hour * 24 * 365,
}

// chooseStep picks a "round" duration step that will produce close to maxNumMarks marks.
func chooseStep(duration time.Duration, maxNumMarks int) time.Duration {
	for _, step := range snaps {
		if int(duration/step) <= maxNumMarks {
			return step
		}
	}

	return snaps[len(snaps)-1]
}

func getXMarksForHistogram(from, to int, numChars int) []int {
	const minCharsDistanceBetweenMarks = 15
	numMarks := numChars / minCharsDistanceBetweenMarks

	fromTime := time.Unix(int64(from), 0).UTC()
	toTime := time.Unix(int64(to), 0).UTC()

	marksTime := getXMarksForTimeRange(fromTime, toTime, numMarks)
	ret := make([]int, 0, len(marksTime))
	for _, v := range marksTime {
		ret = append(ret, int(v.Unix()))
	}

	return ret
}

func snapDataBinsInChartDot(dataBinsInChartDot int) int {
	for _, snap := range snaps {
		snapMinutes := int(snap / time.Minute)

		if dataBinsInChartDot <= snapMinutes {
			return snapMinutes
		}
	}

	return int(snaps[len(snaps)-1] / time.Minute)
}
