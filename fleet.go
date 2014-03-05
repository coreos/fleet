package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/coreos/fleet/third_party/github.com/golang/glog"
	"github.com/coreos/fleet/third_party/github.com/rakyll/globalconf"

	"github.com/coreos/fleet/agent"
	"github.com/coreos/fleet/config"
	"github.com/coreos/fleet/server"
	"github.com/coreos/fleet/sign"
	"github.com/coreos/fleet/version"
)

const (
	DefaultConfigFile = "/etc/fleet/fleet.conf"
)

func main() {
	// We use a FlagSets since glog adds a bunch of flags we do not want to publish
	userset := flag.NewFlagSet("fleet", flag.ExitOnError)
	printVersion := userset.Bool("version", false, "Print the version and exit")
	cfgPath := userset.String("config", "", fmt.Sprintf("Path to config file. Fleet will look for a config at %s by default.", DefaultConfigFile))

	// Initialize logging so we have it set up while parsing config information
	config.UpdateLoggingFlagsFromConfig(flag.CommandLine, &config.Config{})

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
	cfgset.Bool("verify_units", false, "Verify unit file signatures using local SSH identities")
	cfgset.String("authorized_keys_file", sign.DefaultAuthorizedKeysFile, "File containing public SSH keys to be used for signature verification")

	globalconf.Register("", cfgset)
	cfg, err := getConfig(cfgset, *cfgPath)
	if err != nil {
		glog.Error(err.Error())
		syscall.Exit(1)
	}

	checkPermission()

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

func getConfig(flagset *flag.FlagSet, userCfgFile string) (*config.Config, error) {
	opts := globalconf.Options{EnvPrefix: "FLEET_"}

	if userCfgFile != "" {
		// Fail hard if a user-provided config is not usable
		if _, err := os.Stat(userCfgFile); err != nil {
			glog.Errorf("Unable to use config file %s: %v", userCfgFile, err)
			os.Exit(1)
		}

		glog.Infof("Using provided config file %s", userCfgFile)
		opts.Filename = userCfgFile

	} else if _, err := os.Stat(DefaultConfigFile); err == nil {
		glog.Infof("Using default config file %s", DefaultConfigFile)
		opts.Filename = DefaultConfigFile
	} else {
		glog.Infof("Continuing without config file")
	}

	gconf, err := globalconf.NewWithOptions(&opts)
	if err != nil {
		return nil, err
	}

	gconf.ParseSet("", flagset)

	cfg := config.Config{
		Verbosity: (*flagset.Lookup("verbosity")).Value.(flag.Getter).Get().(int),
		EtcdServers: (*flagset.Lookup("etcd_servers")).Value.(flag.Getter).Get().(stringSlice),
		BootId: (*flagset.Lookup("boot_id")).Value.(flag.Getter).Get().(string),
		PublicIP: (*flagset.Lookup("public_ip")).Value.(flag.Getter).Get().(string),
		RawMetadata: (*flagset.Lookup("metadata")).Value.(flag.Getter).Get().(string),
		UnitPrefix: (*flagset.Lookup("unit_prefix")).Value.(flag.Getter).Get().(string),
		AgentTTL: (*flagset.Lookup("agent_ttl")).Value.(flag.Getter).Get().(string),
		VerifyUnits: (*flagset.Lookup("verify_units")).Value.(flag.Getter).Get().(bool),
		AuthorizedKeysFile: (*flagset.Lookup("authorized_keys_file")).Value.(flag.Getter).Get().(string),
	}

	config.UpdateLoggingFlagsFromConfig(flag.CommandLine, &cfg)

	return &cfg, nil
}

func checkPermission() {
	if os.Getuid() != 0 {
		glog.Warningln("Uid", os.Getuid(), "must have permission to run systemd.")
	}
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
