package config

import (
	"flag"
	"strconv"
	"strings"

	"github.com/golang/glog"
)

type Config struct {
	EtcdServers        []string
	EtcdKeyPrefix      string
	PublicIP           string
	Verbosity          int
	RawMetadata        string
	AgentTTL           string
	VerifyUnits        bool
	AuthorizedKeysFile string
}

func (c *Config) Metadata() map[string]string {
	meta := make(map[string]string, 0)

	for _, pair := range strings.Split(c.RawMetadata, ",") {
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
}
