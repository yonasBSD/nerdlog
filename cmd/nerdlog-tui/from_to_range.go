package main

import "github.com/juju/errors"

type FromToRange struct {
	From TimeOrDur
	To   TimeOrDur
}

func ParseFromToRange(s string) (FromToRange, error) {
	// TODO:
	return FromToRange{}, errors.Errorf("not implemented")
}

func (ftr *FromToRange) String() string {
	// TODO: if both From and To are absolute and have the same day,
	// then omit day for the To.
	// It also needs to be supported by ParseFromToRange.
	fromStr := ftr.From.Format(inputTimeLayout)

	if ftr.To.IsZero() {
		return fromStr
	}

	return fromStr + " to " + ftr.To.Format(inputTimeLayout)
}
