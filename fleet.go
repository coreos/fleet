package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/coreos/fleet/config"
	"github.com/coreos/fleet/server"
	log "github.com/coreos/fleet/third_party/github.com/golang/glog"
	// "github.com/coreos/fleet/version"
)

// "Logging level"
// "List of etcd endpoints"
// "Override default BootID of fleet machine"
// "IP address that fleet machine should publish"
// "List of key-value metadata to assign to the fleet machine"
// "Prefix that should be used for all systemd units"
// "TTL in seconds of fleet machine state in etcd"
// "Verify unit file signatures using local SSH identities"
// "File containing public SSH keys to be used for signature verification"

var (
	cfg config.Config
)

func init() {
	flag.IntVar(&cfg.Verbosity, "verbosity", 1, "set log verbosity level")

	es := stringSlice(cfg.EtcdServers)
	flag.Var(&es, "etcd_servers", "ze etcd servas")
	flag.StringVar(&cfg.BootID, "boot_id", "", "")
	flag.StringVar(&cfg.PublicIP, "public_ip", "", "")
	flag.StringVar(&cfg.RawMetadata, "metadata", "", "")
	flag.StringVar(&cfg.UnitPrefix, "unit_prefix", "", "")
	flag.StringVar(&cfg.AgentTTL, "agent_ttl", "", "")
	flag.BoolVar(&cfg.VerifyUnits, "verify_units", true, "")
	flag.StringVar(&cfg.AuthorizedKeysFile, "authorized_keys_file", "", "")
}

func main() {
	log.V(1).Infof("Creating Server")
	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf(err.Error())
	}
	srv.Run()

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
		syscall.SIGTERM: shutdown,
		syscall.SIGINT:  shutdown,
		syscall.SIGUSR1: writeState,
	}

	listenForSignals(signals)
}

func getConfig() {
	config.UpdateLoggingFlagsFromConfig(flag.CommandLine, &cfg)
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
