package inputs

import (
  "github.com/johann8384/libbeat/common"
)

type MothershipConfig struct {
  Enabled            bool
  Port               int
  Flush_interval     *int
  Max_retries        *int
  Type               string
  Sleep_interval     int
}

// Functions to be exported by an input plugin
type InputInterface interface {
  // Initialize the input plugin
  Init(config MothershipConfig) error
  InputType() string
  InputVersion() string
  // Run
  Run(chan common.MapStr) error
}
