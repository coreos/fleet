package config

import (
	"flag"
	"io"
	"strconv"

	"github.com/BurntSushi/toml"
	"github.com/golang/glog"

	"github.com/coreos/coreinit/machine"
)

type Config struct {
	BootId      string   `toml:"bootid"`
	EtcdServers []string `toml:"etcd_servers"`
	Verbosity   int      `toml:"verbosity"`
}

func NewConfig() *Config {
	bootid := machine.ReadLocalBootId()
	conf := Config{BootId: bootid, Verbosity: 0}
	return &conf
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
