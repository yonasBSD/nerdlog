package main

import (
	"strings"

	"github.com/juju/errors"
)

type FromToRange struct {
	From TimeOrDur
	To   TimeOrDur
}

func ParseFromToRange(s string) (FromToRange, error) {
	flds := strings.Fields(s)
	if len(flds) < 1 {
		return FromToRange{}, errors.New("time can't be empty. try -5h")
	}

	var from, to TimeOrDur
	var err error

	from, err = parseAndInferTimeOrDur(inputTimeLayout2, flds[0])
	if err != nil {
		return FromToRange{}, errors.Annotatef(err, "invalid 'from' duration")
	}

	to = TimeOrDur{}

	if len(flds) > 1 {
		if flds[1] != "to" || flds[2] == "" {
			return FromToRange{}, errors.Errorf("invalid time format. try '-2h to -1h'")
		}

		var err error
		to, err = parseAndInferTimeOrDur(inputTimeLayout2, flds[2])
		if err != nil {
			return FromToRange{}, errors.Annotatef(err, "invalid 'to' duration")
		}
	}

	return FromToRange{
		From: from,
		To:   to,
	}, nil
}

func (ftr *FromToRange) String() string {
	// TODO: if both From and To are absolute and have the same day,
	// then omit day for the To.
	// It also needs to be supported by ParseFromToRange.
	fromStr := ftr.From.Format(inputTimeLayout2)

	if ftr.To.IsZero() {
		return fromStr
	}

	return fromStr + " to " + ftr.To.Format(inputTimeLayout2)
}
