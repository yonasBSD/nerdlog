package main

import (
	"sync"
	"time"
)

type Options struct {
	Timezone *time.Location
}

type OptionsShared struct {
	mtx     *sync.Mutex
	options Options
}

func NewOptionsShared(options Options) *OptionsShared {
	return &OptionsShared{
		mtx:     &sync.Mutex{},
		options: options,
	}
}

func (o *OptionsShared) GetTimezone() *time.Location {
	o.mtx.Lock()
	defer o.mtx.Unlock()
	return o.options.Timezone
}

func (o *OptionsShared) GetAll() Options {
	o.mtx.Lock()
	defer o.mtx.Unlock()
	return o.options
}

func (o *OptionsShared) Call(f func(o *Options)) {
	o.mtx.Lock()
	defer o.mtx.Unlock()
	f(&o.options)
}

type OptionMeta struct {
	Get  func(o *Options) string
	Set  func(o *Options, value string) error
	Help string
}

var AllOptions = map[string]*OptionMeta{
	"timezone": { // {{{
		Get: func(o *Options) string {
			return o.Timezone.String()
		},
		Set: func(o *Options, value string) error {
			loc, err := time.LoadLocation(value)
			if err != nil {
				return err
			}

			o.Timezone = loc
			return nil
		},
		Help: "Timezone to use in the UI.",
	}, // }}}
}
