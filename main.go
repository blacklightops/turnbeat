package main

import (
	"flag"
	"fmt"
	"github.com/blacklightops/libbeat/common/droppriv"
	"github.com/blacklightops/libbeat/filters"
	"github.com/blacklightops/libbeat/filters/nop"
	"github.com/blacklightops/libbeat/filters/opentsdb"
	"github.com/blacklightops/libbeat/filters/graphite"
  "github.com/blacklightops/libbeat/filters/jsonexpander"
	"github.com/blacklightops/libbeat/logp"
	"github.com/blacklightops/libbeat/publisher"
	"github.com/blacklightops/turnbeat/config"
	"github.com/blacklightops/turnbeat/reader"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"os/signal"
	"runtime"
)

// You can overwrite these, e.g.: go build -ldflags "-X main.Version 1.0.0-beta3"
var Version = "0.0.1"
var Name = "turnbeat"

var EnabledFilterPlugins map[filters.Filter]filters.FilterPlugin = map[filters.Filter]filters.FilterPlugin{
	filters.NopFilter:      new(nop.Nop),
	filters.OpenTSDBFilter: new(opentsdb.OpenTSDB),
	filters.GraphiteFilter: new(graphite.Graphite),
  filters.JSONExpanderFilter: new(jsonexpander.JSONExpander),
}

func main() {
	// Use our own FlagSet, because some libraries pollute the global one
	var cmdLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	configfile := cmdLine.String("c", "./"+Name+".yml", "Configuration file")
	printVersion := cmdLine.Bool("version", false, "Print version and exit")

	// Adds logging specific flags
	logp.CmdLineFlags(cmdLine)
	publisher.CmdLineFlags(cmdLine)

	cmdLine.Parse(os.Args[1:])
	if *printVersion {
		fmt.Printf("Turnbeat version %s (%s)\n", Version, runtime.GOARCH)
		return
	}

	// configuration file
	filecontent, err := ioutil.ReadFile(*configfile)
	if err != nil {
		fmt.Printf("Fail to read %s: %s. Exiting.\n", *configfile, err)
		os.Exit(1)
	}
	if err = yaml.Unmarshal(filecontent, &config.ConfigSingleton); err != nil {
		fmt.Printf("YAML config parsing failed on %s: %s. Exiting.\n", *configfile, err)
		os.Exit(1)
	}

	logp.Init(Name, &config.ConfigSingleton.Logging)

	logp.Info("Initializing output plugins")
	if err = publisher.Publisher.Init(config.ConfigSingleton.Output, config.ConfigSingleton.Shipper); err != nil {
		logp.Critical(err.Error())
		os.Exit(1)
	}

	stopCb := func() {
	}

	logp.Info("Initializing filter plugins")
	afterInputsQueue, err := filters.FiltersRun(
		config.ConfigSingleton.Filter,
		EnabledFilterPlugins,
		publisher.Publisher.Queue,
		stopCb)
	if err != nil {
		logp.Critical("%v", err)
		os.Exit(1)
	}

	logp.Info("Initializing input plugins")
	if err = reader.Reader.Init(config.ConfigSingleton.Input); err != nil {
		logp.Critical("Critical Error: %v", err)
		os.Exit(1)
	}

	if err = droppriv.DropPrivileges(config.ConfigSingleton.RunOptions); err != nil {
		logp.Critical("Critical Error: %v", err)
		os.Exit(1)
	}

	logp.Info("Starting input plugins")
	go reader.Reader.Run(afterInputsQueue)

	logp.Info("Turnbeat Started")
	signalChan := make(chan os.Signal, 1)
	cleanupDone := make(chan bool)
	signal.Notify(signalChan, os.Interrupt)
	go func() {
		for range signalChan {
			fmt.Println("\nReceived an interrupt, stopping services...\n")
			//cleanup(services, c)
			cleanupDone <- true
		}
	}()
	<-cleanupDone

	//  for {
	//    event := <-afterInputsQueue
	//    reader.Reader.PrintReaderEvent(event)
	//    logp.Debug("events", "Event: %v", event)
	//  }
}
