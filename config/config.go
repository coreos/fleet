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
	TokenLimit              int
	DisableEngine           bool
	DisableWatches          bool
	VerifyUnits             bool
	AuthorizedKeysFile      string
	EnableUnitStateCache    bool
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
