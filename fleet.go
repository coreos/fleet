package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/coreos/fleet/third_party/github.com/coreos/go-etcd/etcd"
	"github.com/coreos/fleet/third_party/github.com/golang/glog"
	"github.com/coreos/fleet/third_party/github.com/rakyll/globalconf"

	"github.com/coreos/fleet/agent"
	"github.com/coreos/fleet/config"
	"github.com/coreos/fleet/server"
	"github.com/coreos/fleet/version"
)

func main() {
	// We use a FlagSets since glog adds a bunch of flags we do not want to publish
	userset := flag.NewFlagSet("fleet", flag.ExitOnError)
	printVersion := userset.Bool("version", false, "Print the version and exit")
	cfgPath := userset.String("config", "/etc/fleet/fleet.conf", "Path to config file")

	err := userset.Parse(os.Args[1:])
	if err == flag.ErrHelp {
		userset.Usage()
		syscall.Exit(1)
	}

	if *printVersion {
		fmt.Println("fleet version", version.Version)
		os.Exit(0)
	}

	cfgset := flag.NewFlagSet("fleet", flag.ExitOnError)
	cfgset.Int("verbosity", 0, "Logging level")
	cfgset.Var(&stringSlice{}, "etcd_servers", "List of etcd endpoints")
	cfgset.String("boot_id", "", "Override default BootID of fleet machine")
	cfgset.String("public_ip", "", "IP address that fleet machine should publish")
	cfgset.String("metadata", "", "List of key-value metadata to assign to the fleet machine")
	cfgset.String("unit_prefix", "", "Prefix that should be used for all systemd units")
	cfgset.String("agent_ttl", agent.DefaultTTL, "TTL in seconds of fleet machine state in etcd")

	globalconf.Register("", cfgset)
	cfg, err := getConfig(cfgset, *cfgPath)
	if err != nil {
		glog.Errorf(err)
		syscall.Exit(1)
	}

	config.UpdateLoggingFlagsFromConfig(cfg)
	etcd.SetLogger(etcdLogger{})

	srv := server.New(*cfg)
	srv.Run()

	reconfigure := func() {
		glog.Infof("Reloading configuration from %s", *cfgPath)

		cfg, err := getConfig(cfgset, *cfgPath)
		if err != nil {
			glog.Errorf(err.Error())
			syscall.Exit(1)
		}

		srv.Stop()

		config.UpdateLoggingFlagsFromConfig(cfg)
		srv = server.New(*cfg)

		srv.Run()
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

func getConfig(flagset *flag.FlagSet, file string) (*config.Config, error) {
	globalconf.EnvPrefix = "FLEET_"

	gconf, err := globalconf.NewWithFilename(file)
	if err != nil {
		return nil, err
	}

	gconf.ParseSet("", flagset)

	cfg := config.NewConfig()
	cfg.Verbosity = (*flagset.Lookup("verbosity")).Value.(flag.Getter).Get().(int)
	cfg.EtcdServers = (*flagset.Lookup("etcd_servers")).Value.(flag.Getter).Get().(stringSlice)
	cfg.BootId = (*flagset.Lookup("boot_id")).Value.(flag.Getter).Get().(string)
	cfg.PublicIP = (*flagset.Lookup("public_ip")).Value.(flag.Getter).Get().(string)
	cfg.RawMetadata = (*flagset.Lookup("metadata")).Value.(flag.Getter).Get().(string)
	cfg.UnitPrefix = (*flagset.Lookup("unit_prefix")).Value.(flag.Getter).Get().(string)
	cfg.AgentTTL = (*flagset.Lookup("agent_ttl")).Value.(flag.Getter).Get().(string)

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

type stringSlice []string

func (f *stringSlice) Set(value string) error {
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimLeft(item, " [\"")
		item = strings.TrimRight(item, " \"]")
		*f = append(*f, item)
	}

	return nil
}

func (f *stringSlice) String() string {
	return fmt.Sprintf("%v", *f)
}

func (f *stringSlice) Value() []string {
	return *f
}

func (f *stringSlice) Get() interface{} {
	return *f
}
