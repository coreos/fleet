package config

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/coreos/fleet/third_party/github.com/coreos/go-etcd/etcd"
	"github.com/coreos/fleet/third_party/github.com/golang/glog"
)

type Config struct {
	BootID             string
	EtcdServers        []string
	PublicIP           string
	Verbosity          int
	RawMetadata        string
	AgentTTL           string
	VerifyUnits        bool
	AuthorizedKeysFile string
}

func (self *Config) Metadata() map[string]string {
	meta := make(map[string]string, 0)

	for _, pair := range strings.Split(self.RawMetadata, ",") {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		meta[key] = val
	}

	return meta
}

// UpdateLoggingFlagsFromConfig extracts the logging-related options from
// the provided config and sets flags in the given flagset
func UpdateLoggingFlagsFromConfig(flagset *flag.FlagSet, conf *Config) {
	err := flagset.Lookup("v").Value.Set(strconv.Itoa(conf.Verbosity))
	if err != nil {
		glog.Errorf("Failed to apply config.Verbosity to flag.v: %v", err)
	}

	err = flagset.Lookup("logtostderr").Value.Set("true")
	if err != nil {
		glog.Errorf("Failed to set flag.logtostderr to true: %v", err)
	}

	if conf.Verbosity > 2 {
		etcd.SetLogger(log.New(os.Stdout, "go-etcd", log.LstdFlags))
	} else {
		etcd.SetLogger(log.New(ioutil.Discard, "go-etcd", log.LstdFlags))
	}
}
