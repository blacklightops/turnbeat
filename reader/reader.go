package reader

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/blacklightops/libbeat/common"
	"github.com/blacklightops/libbeat/logp"
	"github.com/blacklightops/turnbeat/inputs"
	"github.com/blacklightops/turnbeat/inputs/null"
	"github.com/blacklightops/turnbeat/inputs/packages"
	"github.com/blacklightops/turnbeat/inputs/procfs"
	"github.com/blacklightops/turnbeat/inputs/syslog"
	"github.com/blacklightops/turnbeat/inputs/tail"
	"github.com/blacklightops/turnbeat/inputs/tcp"
	"github.com/blacklightops/turnbeat/inputs/udp"
	"github.com/blacklightops/turnbeat/inputs/redis"
	"strings"
)

type ReaderType struct {
	name     string
	tags     []string
	disabled bool
	Index    string
	Input    []inputs.InputInterface
	Queue    chan common.MapStr
}

var Reader ReaderType

/***********************************/
/* register new input plugins here */
/***********************************/
func newInputInstance(name string) inputs.InputInterface {
	logp.Info("creating new instance of type %s", name)
	switch name {
	case "tcp":
		return new(tcp.TcpInput)
	case "udp":
		return new(udp.UdpInput)
	case "tail":
		return new(tail.TailInput)
	case "syslog":
		return new(syslog.SyslogInput)
	case "procfs":
		return new(procfs.ProcfsInput)
	case "packages":
		return new(packages.PackagesInput)
	case "redis":
		return new(redis.RedisInput)
	case "null":
		return new(null.NullInput)
	}
	return nil
}

func (reader *ReaderType) PrintReaderEvent(event common.MapStr) {
	json, err := json.MarshalIndent(event, "", "  ")
	if err != nil {
		logp.Err("json.Marshal: %s", err)
	} else {
		logp.Debug("reader", "Reader: %s", string(json))
	}
}

func (reader *ReaderType) Init(inputMap map[string]inputs.MothershipConfig) error {
	logp.Info("reader input config", inputMap)

	var globalConf inputs.MothershipConfig

	for inputId, config := range inputMap {
		// default instance 0
		inputName, instance := inputId, "0"
		if strings.Contains(inputId, "_") {
			// otherwise grok tcp_2 as inputName = tcp, instance = 2
			sv := strings.Split(inputId, "_")
			inputName, instance = sv[0], sv[1]
		}
		logp.Info(fmt.Sprintf("input type: %s instance: %s\n", inputName, instance))
		logp.Debug("reader", "instance config: %s", config)

		// handling for "global" config section
		if inputName == "global" {
			logp.Info("global input configuration read")
			globalConf = config
		}

		plugin := newInputInstance(inputName)
		if plugin != nil && config.Enabled {
			config.Normalize(globalConf)
			err := plugin.Init(config)
			if err != nil {
				logp.Err("Fail to initialize %s plugin as input: %s", inputName, err)
				return err
			} else {
				logp.Info("Initialized %s plugin as input", inputName)
			}
			reader.Input = append(reader.Input, plugin)
		}
	}

	if len(reader.Input) == 0 {
		logp.Info("No inputs are defined. Please define one under the input section.")
		return errors.New("No input are defined. Please define one under the input section.")
	} else {
		logp.Info("%d inputs defined", len(reader.Input))
	}

	return nil
}

func (reader *ReaderType) Run(output chan common.MapStr) error {
	logp.Info("Attempting to start %d inputs", len(reader.Input))

	for _, plugin := range reader.Input {
		err := plugin.Run(output)
		if err != nil {
			logp.Err("Fail to start input plugin %s : %s", plugin.InputType(), err)
			return err
		} else {
			logp.Info("Started input plugin %s", plugin.InputType())
		}
	}
	return nil
}
