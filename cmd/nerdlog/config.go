package main

import (
	"io/ioutil"
	"os"
	"sort"

	"github.com/dimonomid/nerdlog/core"
	"github.com/juju/errors"
	"gopkg.in/yaml.v2"
)

type ConfigLogStreams struct {
	LogStreams core.ConfigLogStreams `yaml:"log_streams"`
}

func LoadLogstreamsConfigFromFile(path string) (*ConfigLogStreams, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.Annotatef(err, "opening config file: %s", path)
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, errors.Annotatef(err, "reading config file %s", path)
	}

	var cfg ConfigLogStreams
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, errors.Annotatef(err, "unmarshaling yaml from %s", path)
	}

	// Make sure the logstreams configuration is not obviously invalid.
	for k, cls := range cfg.LogStreams {
		_, ok := core.ValidSudoModes[cls.Options.SudoMode]
		if cls.Options.SudoMode != "" && !ok {
			validModes := make([]string, 0, len(core.ValidSudoModes))
			for mode := range core.ValidSudoModes {
				validModes = append(validModes, string(mode))
			}

			sort.Strings(validModes)

			return nil, errors.Errorf(
				"%s: invalid sudo_mode %q; valid options are: %s",
				k, cls.Options.SudoMode, validModes,
			)
		}

		if cls.Options.SudoMode != "" && cls.Options.Sudo {
			return nil, errors.Errorf(
				"%s: both sudo and sudo_mode are set; please only use one of them", k,
			)
		}
	}

	return &cfg, nil
}
