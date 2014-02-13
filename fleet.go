package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/coreos/fleet/third_party/github.com/coreos/go-etcd/etcd"
	"github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/config"
	"github.com/coreos/fleet/server"
	"github.com/coreos/fleet/version"
)

func main() {
	// We use a custom FlagSet since golang/glog adds a bunch of flags we
	// do not want to publish
	flagset := flag.NewFlagSet("fleet", flag.ExitOnError)
	printVersion := flagset.Bool("version", false, "Prints the version.")
	cfgPath := flagset.String("config_file", "", "Path to config file.")
	err := flagset.Parse(os.Args[1:])

	// We do this manually since we're using a custom FlagSet
	if err == flag.ErrHelp {
		flag.Usage()
		syscall.Exit(1)
	}

	if *printVersion {
		fmt.Println("fleet version", version.Version)
		os.Exit(0)
	}

	// Print out to stderr by default (stderr instead of stdout due to glog's choices)
	flag.Lookup("logtostderr").Value.Set("true")

	cfg, err := loadConfigFromPath(*cfgPath)
	if err != nil {
		glog.Errorf(err.Error())
		syscall.Exit(1)
	}

	etcd.SetLogger(etcdLogger{})

	srv := server.New(*cfg)
	srv.Run()

	reconfigure := func() {
		glog.Infof("Reloading config file from %s", *cfgPath)
		cfg, err := loadConfigFromPath(*cfgPath)
		if err != nil {
			glog.Errorf(err.Error())
			syscall.Exit(1)
		} else {
			srv.Stop()
			srv = server.New(*cfg)
			srv.Run()
		}
	}

	shutdown := func() {
		glog.Infof("Gracefully shutting down")
		srv.Stop()
		srv.Purge()
		syscall.Exit(0)
	}

	signals := map[os.Signal]func(){
		syscall.SIGHUP:  reconfigure,
		syscall.SIGTERM: shutdown,
		syscall.SIGINT:  shutdown,
	}

	listenForSignals(signals)
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

func listenForSignals(sigmap map[os.Signal]func()) {
	sigchan := make(chan os.Signal, 1)

	for k, _ := range sigmap {
		signal.Notify(sigchan, k)
	}

	for true {
		sig := <-sigchan
		handler, ok := sigmap[sig]
		if ok {
			handler()
		}
	}
}

type etcdLogger struct{}

func (el etcdLogger) Debug(args ...interface{}) {
	glog.V(3).Info(args...)
}

func (el etcdLogger) Debugf(fmt string, args ...interface{}) {
	glog.V(3).Infof(fmt, args...)
}

func (el etcdLogger) Warning(args ...interface{}) {
	glog.Warning(args...)
}

func (el etcdLogger) Warningf(fmt string, args ...interface{}) {
	glog.Warningf(fmt, args...)
}
