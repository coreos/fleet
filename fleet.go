package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"
	"github.com/coreos/fleet/Godeps/_workspace/src/github.com/rakyll/globalconf"

	"github.com/coreos/fleet/agent"
	"github.com/coreos/fleet/config"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/server"
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
		os.Exit(1)
	}

	if *printVersion {
		fmt.Println("fleet version", version.Version)
		os.Exit(0)
	}

	cfgset := flag.NewFlagSet("fleet", flag.ExitOnError)
	cfgset.Int("verbosity", 0, "Logging level")
	cfgset.Var(&stringSlice{}, "etcd_servers", "List of etcd endpoints")
	cfgset.String("etcd_keyfile", "", "SSL key file used to secure etcd communication")
	cfgset.String("etcd_certfile", "", "SSL certification file used to secure etcd communication")
	cfgset.String("etcd_cafile", "", "SSL Certificate Authority file used to secure etcd communication")
	cfgset.String("etcd_key_prefix", registry.DefaultKeyPrefix, "Keyspace for fleet data in etcd")
	cfgset.Float64("etcd_request_timeout", 1.0, "Amount of time in seconds to allow a single etcd request before considering it failed.")
	cfgset.Float64("engine_reconcile_interval", 2.0, "Interval at which the engine should reconcile the cluster schedule in etcd.")
	cfgset.String("public_ip", "", "IP address that fleet machine should publish")
	cfgset.String("metadata", "", "List of key-value metadata to assign to the fleet machine")
	cfgset.String("agent_ttl", agent.DefaultTTL, "TTL in seconds of fleet machine state in etcd")
	cfgset.Bool("verify_units", false, "DEPRECATED - This option is ignored")
	cfgset.String("authorized_keys_file", "", "DEPRECATED - This option is ignored")

	globalconf.Register("", cfgset)
	cfg, err := getConfig(cfgset, *cfgPath)
	if err != nil {
		log.Fatalf(err.Error())
	}

	log.V(1).Infof("Creating Server")
	srv, err := server.New(*cfg)
	if err != nil {
		log.Fatalf("Failed creating Server: %v", err.Error())
	}
	srv.Run()

	reconfigure := func() {
		log.Infof("Reloading configuration from %s", *cfgPath)

		cfg, err := getConfig(cfgset, *cfgPath)
		if err != nil {
			log.Fatalf(err.Error())
		}

		log.Infof("Restarting server components")
		srv.Stop()

		srv, err = server.New(*cfg)
		if err != nil {
			log.Fatalf(err.Error())
		}
		srv.Run()
	}

	shutdown := func() {
		log.Infof("Gracefully shutting down")
		srv.Stop()
		srv.Purge()
		os.Exit(0)
	}

	writeState := func() {
		log.Infof("Dumping server state")

		encoded, err := json.Marshal(srv)
		if err != nil {
			log.Errorf("Failed to dump server state: %v", err)
			return
		}

		if _, err := os.Stdout.Write(encoded); err != nil {
			log.Errorf("Failed to dump server state: %v", err)
			return
		}

		os.Stdout.Write([]byte("\n"))

		log.V(1).Infof("Finished dumping server state")
	}

	signals := map[os.Signal]func(){
		syscall.SIGHUP:  reconfigure,
		syscall.SIGTERM: shutdown,
		syscall.SIGINT:  shutdown,
		syscall.SIGUSR1: writeState,
	}

	listenForSignals(signals)
}

func getConfig(flagset *flag.FlagSet, userCfgFile string) (*config.Config, error) {
	opts := globalconf.Options{EnvPrefix: "FLEET_"}

	if userCfgFile != "" {
		// Fail hard if a user-provided config is not usable
		fi, err := os.Stat(userCfgFile)
		if err != nil {
			log.Fatalf("Unable to use config file %s: %v", userCfgFile, err)
		}
		if fi.IsDir() {
			log.Fatalf("Provided config %s is a directory, not a file", userCfgFile)
		}

		log.Infof("Using provided config file %s", userCfgFile)
		opts.Filename = userCfgFile

	} else if _, err := os.Stat(DefaultConfigFile); err == nil {
		log.Infof("Using default config file %s", DefaultConfigFile)
		opts.Filename = DefaultConfigFile
	} else {
		log.Infof("No provided or default config file found - proceeding without")
	}

	gconf, err := globalconf.NewWithOptions(&opts)
	if err != nil {
		return nil, err
	}

	gconf.ParseSet("", flagset)

	cfg := config.Config{
		Verbosity:               (*flagset.Lookup("verbosity")).Value.(flag.Getter).Get().(int),
		EtcdServers:             (*flagset.Lookup("etcd_servers")).Value.(flag.Getter).Get().(stringSlice),
		EtcdKeyPrefix:           (*flagset.Lookup("etcd_key_prefix")).Value.(flag.Getter).Get().(string),
		EtcdKeyFile:             (*flagset.Lookup("etcd_keyfile")).Value.(flag.Getter).Get().(string),
		EtcdCertFile:            (*flagset.Lookup("etcd_certfile")).Value.(flag.Getter).Get().(string),
		EtcdCAFile:              (*flagset.Lookup("etcd_cafile")).Value.(flag.Getter).Get().(string),
		EtcdRequestTimeout:      (*flagset.Lookup("etcd_request_timeout")).Value.(flag.Getter).Get().(float64),
		EngineReconcileInterval: (*flagset.Lookup("engine_reconcile_interval")).Value.(flag.Getter).Get().(float64),
		PublicIP:                (*flagset.Lookup("public_ip")).Value.(flag.Getter).Get().(string),
		RawMetadata:             (*flagset.Lookup("metadata")).Value.(flag.Getter).Get().(string),
		AgentTTL:                (*flagset.Lookup("agent_ttl")).Value.(flag.Getter).Get().(string),
		VerifyUnits:             (*flagset.Lookup("verify_units")).Value.(flag.Getter).Get().(bool),
		AuthorizedKeysFile:      (*flagset.Lookup("authorized_keys_file")).Value.(flag.Getter).Get().(string),
	}

	if cfg.VerifyUnits {
		log.Error("Config option verify_units is no longer supported - ignoring")
	}
	if len(cfg.AuthorizedKeysFile) > 0 {
		log.Error("Config option authorized_keys_file is no longer supported - ignoring")
	}

	config.UpdateLoggingFlagsFromConfig(flag.CommandLine, &cfg)

	return &cfg, nil
}

func listenForSignals(sigmap map[os.Signal]func()) {
	sigchan := make(chan os.Signal, 1)

	for k := range sigmap {
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
