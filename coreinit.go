package main

import (
	"flag"
	"os"
	"syscall"

	"github.com/coreos/go-etcd/etcd"
	"github.com/golang/glog"

	"github.com/coreos/coreinit/agent"
	"github.com/coreos/coreinit/config"
	"github.com/coreos/coreinit/engine"
	"github.com/coreos/coreinit/machine"
	"github.com/coreos/coreinit/registry"
)

func main() {
	var configPath string

	flag.StringVar(&configPath, "config_file", "", "Path to config file.")
	flag.Parse()

	conf := config.NewConfig()

	if configPath != "" {
		configFile, err := os.Open(configPath)
		if err != nil {
			glog.Fatalf("Unable to open config file at %s: %s", configPath, err)
			syscall.Exit(1)
		}

		err = config.UpdateConfigFromFile(conf, configFile)
		if err != nil {
			glog.Fatalf("Failed to parse config file at %s: %s", configPath, err)
			syscall.Exit(1)
		}
	}

	config.UpdateFlagsFromConfig(conf)

	if glog.V(2) {
		etcd.OpenDebug()
	}

	m := machine.New(conf.BootId)
	r := registry.New()
	es := registry.NewEventStream()

	a := agent.New(r, es, m, "")
	go a.Run()

	e := engine.New(r, es, m)
	e.Run()
}
