// Copyright 2014 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/rakyll/globalconf"

	"github.com/coreos/fleet/agent"
	"github.com/coreos/fleet/config"
	"github.com/coreos/fleet/log"
	"github.com/coreos/fleet/pkg"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/server"
	"github.com/coreos/fleet/version"
)

const (
	DefaultConfigFile = "/etc/fleet/fleet.conf"
	FleetdDescription = "fleetd is the server component of fleet, a simple orchestration system for scheduling systemd units in a cluster."
)

func main() {
	userset := flag.NewFlagSet("fleet", flag.ExitOnError)
	printVersion := userset.Bool("version", false, "Print the version and exit")
	cfgPath := userset.String("config", "", fmt.Sprintf("Path to config file. Fleet will look for a config at %s by default.", DefaultConfigFile))

	userset.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s\nUsage of %s:\n", FleetdDescription, os.Args[0])
		userset.PrintDefaults()
	}

	err := userset.Parse(os.Args[1:])
	if err != nil {
		userset.Usage()
		os.Exit(1)
	}

	args := userset.Args()
	if len(args) > 0 {
		// support `fleetd version` the same as `fleetd --version`
		if args[0] == "version" {
			*printVersion = true
		} else {
			fmt.Fprintf(os.Stderr, "%s takes no arguments. Did you mean to invoke fleetctl instead?\n", os.Args[0])
			userset.Usage()
			os.Exit(1)
		}
	}

	if *printVersion {
		fmt.Println("fleetd version", version.Version)
		os.Exit(0)
	}

	log.Infof("Starting fleetd version %v", version.Version)

	cfgset := flag.NewFlagSet("fleet", flag.ExitOnError)
	cfgset.Int("verbosity", 0, "Logging level")
	cfgset.Var(&pkg.StringSlice{"http://127.0.0.1:2379", "http://127.0.0.1:4001"}, "etcd_servers", "List of etcd endpoints")
	cfgset.String("etcd_keyfile", "", "SSL key file used to secure etcd communication")
	cfgset.String("etcd_certfile", "", "SSL certification file used to secure etcd communication")
	cfgset.String("etcd_cafile", "", "SSL Certificate Authority file used to secure etcd communication")
	cfgset.String("etcd_username", "", "Username for etcd authentication")
	cfgset.String("etcd_password", "", "Password for etcd authentication")
	cfgset.String("etcd_key_prefix", registry.DefaultKeyPrefix, "Keyspace for fleet data in etcd")
	cfgset.Float64("etcd_request_timeout", 1.0, "Amount of time in seconds to allow a single etcd request before considering it failed.")
	cfgset.Float64("engine_reconcile_interval", 2.0, "Interval at which the engine should reconcile the cluster schedule in etcd.")
	cfgset.String("public_ip", "", "IP address that fleet machine should publish")
	cfgset.String("metadata", "", "List of key-value metadata to assign to the fleet machine")
	cfgset.String("agent_ttl", agent.DefaultTTL, "TTL in seconds of fleet machine state in etcd")
	cfgset.Int("token_limit", 100, "Maximum number of entries per page returned from API requests")
	cfgset.Bool("disable_engine", false, "Disable the engine entirely, use with care")
	cfgset.Bool("disable_watches", false, "Disable the use of etcd watches. Increases scheduling latency")
	cfgset.Bool("verify_units", false, "DEPRECATED - This option is ignored")
	cfgset.String("authorized_keys_file", "", "DEPRECATED - This option is ignored")

	globalconf.Register("", cfgset)
	cfg, err := getConfig(cfgset, *cfgPath)
	if err != nil {
		log.Fatalf(err.Error())
	}

	log.Debugf("Creating Server")
	srv, err := server.New(*cfg, nil)
	if err != nil {
		log.Fatalf("Failed creating Server: %v", err.Error())
	}
	srv.Run()

	srvMutex := sync.Mutex{}

	reconfigure := func() {
		log.Infof("Reloading configuration from %s", *cfgPath)

		srvMutex.Lock()
		defer srvMutex.Unlock()

		cfg, err := getConfig(cfgset, *cfgPath)
		if err != nil {
			log.Fatalf(err.Error())
		}

		log.Infof("Restarting server components")
		srv.SetReconfigServer(true)

		// Get Server.listeners[] to keep it for a new server,
		// before killing the old server.
		oldListeners := srv.GetApiServerListeners()

		srv.Kill()

		// The new server takes the original listeners.
		srv, err = server.New(*cfg, oldListeners)
		if err != nil {
			log.Fatalf(err.Error())
		}

		srv.Run()
		srv.SetReconfigServer(false)
	}

	shutdown := func() {
		log.Infof("Gracefully shutting down")

		srvMutex.Lock()
		defer srvMutex.Unlock()

		srv.Kill()
		srv.Purge()
		os.Exit(0)
	}

	writeState := func() {
		log.Infof("Dumping server state")

		srvMutex.Lock()
		defer srvMutex.Unlock()

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

		log.Debugf("Finished dumping server state")
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
		EtcdServers:             (*flagset.Lookup("etcd_servers")).Value.(flag.Getter).Get().(pkg.StringSlice),
		EtcdKeyPrefix:           (*flagset.Lookup("etcd_key_prefix")).Value.(flag.Getter).Get().(string),
		EtcdKeyFile:             (*flagset.Lookup("etcd_keyfile")).Value.(flag.Getter).Get().(string),
		EtcdCertFile:            (*flagset.Lookup("etcd_certfile")).Value.(flag.Getter).Get().(string),
		EtcdCAFile:              (*flagset.Lookup("etcd_cafile")).Value.(flag.Getter).Get().(string),
		EtcdUsername:            (*flagset.Lookup("etcd_username")).Value.(flag.Getter).Get().(string),
		EtcdPassword:            (*flagset.Lookup("etcd_password")).Value.(flag.Getter).Get().(string),
		EtcdRequestTimeout:      (*flagset.Lookup("etcd_request_timeout")).Value.(flag.Getter).Get().(float64),
		EngineReconcileInterval: (*flagset.Lookup("engine_reconcile_interval")).Value.(flag.Getter).Get().(float64),
		PublicIP:                (*flagset.Lookup("public_ip")).Value.(flag.Getter).Get().(string),
		RawMetadata:             (*flagset.Lookup("metadata")).Value.(flag.Getter).Get().(string),
		AgentTTL:                (*flagset.Lookup("agent_ttl")).Value.(flag.Getter).Get().(string),
		DisableEngine:           (*flagset.Lookup("disable_engine")).Value.(flag.Getter).Get().(bool),
		DisableWatches:          (*flagset.Lookup("disable_watches")).Value.(flag.Getter).Get().(bool),
		VerifyUnits:             (*flagset.Lookup("verify_units")).Value.(flag.Getter).Get().(bool),
		TokenLimit:              (*flagset.Lookup("token_limit")).Value.(flag.Getter).Get().(int),
		AuthorizedKeysFile:      (*flagset.Lookup("authorized_keys_file")).Value.(flag.Getter).Get().(string),
	}

	if cfg.VerifyUnits {
		log.Error("Config option verify_units is no longer supported - ignoring")
	}
	if len(cfg.AuthorizedKeysFile) > 0 {
		log.Error("Config option authorized_keys_file is no longer supported - ignoring")
	}

	if cfg.Verbosity > 0 {
		log.EnableDebug()
	}

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
