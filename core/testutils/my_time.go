package testutils

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v2"
)

type MyTime struct {
	time.Time
}

var _ yaml.Unmarshaler = &MyTime{}

func (mt *MyTime) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var str string
	if err := unmarshal(&str); err != nil {
		return err
	}

	if str == "" {
		mt.Time = time.Time{}
		return nil
	}

	layouts := []string{
		"2006-01-02T15:04:05.999999999Z07:00",
	}

	var err error
	for _, layout := range layouts {
		var t time.Time
		t, err = time.Parse(layout, str)
		if err == nil {
			mt.Time = t
			return nil
		}
	}

	return fmt.Errorf("invalid time format: %q", str)
}
