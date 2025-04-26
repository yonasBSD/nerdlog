package main

import (
	"io/ioutil"
	"os"

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

	return &cfg, nil
}
