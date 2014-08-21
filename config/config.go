package config

import (
	"strings"
)

type Config struct {
	EtcdServers             []string
	EtcdKeyPrefix           string
	EtcdKeyFile             string
	EtcdCertFile            string
	EtcdCAFile              string
	EtcdRequestTimeout      float64
	EngineReconcileInterval float64
	PublicIP                string
	Verbosity               int
	RawMetadata             string
	AgentTTL                string
	VerifyUnits             bool
	AuthorizedKeysFile      string
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
