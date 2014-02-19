package config

import (
	"flag"
	"strconv"
	"strings"

	"github.com/coreos/fleet/third_party/github.com/golang/glog"
)

type Config struct {
	BootId      string
	EtcdServers []string
	PublicIP    string
	Verbosity   int
	RawMetadata string
	UnitPrefix  string
	AgentTTL    string
}

func NewConfig() *Config {
	conf := Config{BootId: "", Verbosity: 0, PublicIP: ""}
	return &conf
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

func UpdateLoggingFlagsFromConfig(conf *Config) {
	err := flag.Lookup("v").Value.Set(strconv.Itoa(conf.Verbosity))
	if err != nil {
		glog.Errorf("Failed to apply config.Verbosity to flag.v: %v", err)
	}

	err = flag.Lookup("logtostderr").Value.Set("true")
	if err != nil {
		glog.Errorf("Failed to set flag.logtostderr to true: %v", err)
	}
}
