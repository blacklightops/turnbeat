package inputs

import (
	"github.com/blacklightops/libbeat/common"
	"time"
)

type MothershipConfig struct {
	Enabled        bool
	Host           string
	Port           int
	Key	           string
	DB             int
	Flush_interval *int
	Max_retries    *int
	Type           string
	Sleep_interval int
	Tick_interval  int
	Minor_interval int
	Major_interval int
	Filename       string
}

// Functions to be exported by an input plugin
type InputInterface interface {
	// Initialize the input plugin
	Init(config MothershipConfig) error
	// Be able to retrieve the config
	GetConfig() MothershipConfig
	InputType() string
	InputVersion() string
	// Run
	Run(chan common.MapStr) error
}

// Normalize a config by pulling in global settings if appropriate, and enforcing absolute minimums
func (l *MothershipConfig) Normalize(global MothershipConfig) {
	// default to global if none assigned
	if l.Tick_interval == 0 {
		l.Tick_interval = global.Tick_interval
	}
	if l.Minor_interval == 0 {
		l.Minor_interval = global.Minor_interval
	}
	if l.Tick_interval == 0 {
		l.Major_interval = global.Major_interval
	}

	// adjust intervals to minimums if too low
	if l.Tick_interval < 15 {
		l.Tick_interval = 15
	}
	if l.Minor_interval < 60 {
		l.Minor_interval = 60
	}
	if l.Major_interval < 900 {
		l.Major_interval = 900
	}
}

// PeriodicTaskRunner and friends
type taskRunner func(chan common.MapStr)

// empty function useful for passing into PeriodicTaskRunner sometimes
func EmptyFunc(chan common.MapStr) {}

func PeriodicTaskRunner(l InputInterface, output chan common.MapStr, ti taskRunner, mi taskRunner, ma taskRunner) {
	mi_last := time.Now()
	ma_last := time.Now()
	config := l.GetConfig()

	for {
		ti(output)
		time.Sleep(time.Duration(config.Tick_interval) * time.Second)
		if time.Since(mi_last) > time.Duration(config.Minor_interval)*time.Second {
			mi(output)
			mi_last = time.Now()
		}
		if time.Since(ma_last) > time.Duration(config.Major_interval)*time.Second {
			ma(output)
			ma_last = time.Now()
		}
	}
}
