package config

import (
	"flag"
	"io"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/golang/glog"
)

type Config struct {
	BootId      string   `toml:"bootid"`
	EtcdServers []string `toml:"etcd_servers"`
	PublicIP    string   `toml:"public_ip"`
	Verbosity   int      `toml:"verbosity"`
	RawMetadata string   `toml:"metadata"`
	UnitPrefix  string   `toml:"unit_prefix"`
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

func UpdateConfigFromFile(conf *Config, f io.Reader) error {
	_, err := toml.DecodeReader(f, conf)
	if err != nil {
		return err
	}

	return nil
}

func UpdateFlagsFromConfig(conf *Config) {
	err := flag.Lookup("v").Value.Set(strconv.Itoa(conf.Verbosity))
	if err != nil {
		glog.Errorf("Failed to apply config.Verbosity to flag.v: %s", err.Error())
	}
}
