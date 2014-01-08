package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/coreos/go-etcd/etcd"
	"github.com/golang/glog"

	"github.com/coreos/coreinit/config"
	"github.com/coreos/coreinit/server"
)

func main() {
	// We use a custom FlagSet since golang/glog adds a bunch of flags we
	// do not want to publish
	flagset := flag.NewFlagSet("coreinit", flag.ExitOnError)
	cfgPath := flagset.String("config_file", "", "Path to config file.")
	err := flagset.Parse(os.Args[1:])

	// We do this manually since we're using a custom FlagSet
	if err == flag.ErrHelp {
		flag.Usage()
		syscall.Exit(1)
	}

	// Print out to stderr by default (stderr instead of stdout due to glog's choices)
	flag.Lookup("logtostderr").Value.Set("true")

	cfg, err := loadConfigFromPath(*cfgPath)
	if err != nil {
		glog.Errorf(err.Error())
		syscall.Exit(1)
	}

	if cfg.Verbosity >= 3 {
		etcd.OpenDebug()
	}

	srv := server.New(*cfg)
	srv.Run()

	reconfigure := func() {
		glog.Infof("Reloading config file from %s", *cfgPath)
		cfg, err := loadConfigFromPath(*cfgPath)
		if err != nil {
			glog.Errorf(err.Error())
			syscall.Exit(1)
		} else {
			srv.Configure(cfg)
		}
	}

	listenForSignal(syscall.SIGHUP, reconfigure)
}

func loadConfigFromPath(cp string) (*config.Config, error) {
	cfg := config.NewConfig()

	if cp != "" {
		cfgFile, err := os.Open(cp)
		if err != nil {
			msg := fmt.Sprintf("Unable to open config file at %s: %s", cp, err)
			return nil, errors.New(msg)
		}

		err = config.UpdateConfigFromFile(cfg, cfgFile)
		if err != nil {
			msg := fmt.Sprintf("Failed to parse config file at %s: %s", cp, err)
			return nil, errors.New(msg)
		}
	}

	config.UpdateFlagsFromConfig(cfg)
	return cfg, nil
}

func listenForSignal(sig os.Signal, handler func()) {
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, sig)

	for true {
		<-sigchan
		handler()
	}
}
