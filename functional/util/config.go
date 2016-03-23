// Copyright 2015 CoreOS, Inc.
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

package util

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"text/template"
)

const (
	fleetAPIPort = 54728
	cloudConfig  = `#cloud-config

write_files:
 - path: /opt/fleet/fleet.conf
   content: |
    verbosity=2
    etcd_servers=[{{printf "%q" .EtcdEndpoint}}]
    etcd_key_prefix={{.EtcdKeyPrefix}}
    public_ip={{.IP}}
    agent_ttl=3s

ssh_authorized_keys:
 - {{printf "%q" .PublicKey}}

coreos:
 units:
  - name: 00-local.network
    content: |
      [Match]
      Name=host0
      [Network]
      Address={{.IP}}/16
  - name: fleet.socket
    command: start
  - name: fleet-tcp.socket
    command: start
    content: |
     [Socket]
     ListenStream={{printf "%d" .FleetAPIPort}}
     Service=fleet.service
  - name: fleet.service
    command: start
    content: |
     [Service]
     Environment=FLEET_METADATA=hostname=%H
     ExecStart=/opt/fleet/fleetd -config /opt/fleet/fleet.conf
`
)

var (
	fleetdBinPath  string
	publicKeyPath  string
	configTemplate *template.Template
)

type configValues struct {
	IP            string
	PublicKey     string
	Fleetd        string
	EtcdEndpoint  string
	EtcdKeyPrefix string
	FleetAPIPort  int
}

func init() {
	fleetdBinPath = os.Getenv("FLEETD_BIN")
	if fleetdBinPath == "" {
		fmt.Println("FLEETD_BIN environment variable must be set")
		os.Exit(1)
	} else if _, err := os.Stat(fleetdBinPath); err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
	publicKeyPath = path.Join("fixtures", "id_rsa.pub")
	if _, err := os.Stat(publicKeyPath); err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}

	// sanity check that go's use of tabs didn't leak into the yaml
	if strings.ContainsRune(cloudConfig, '\t') {
		panic("Aagh, no! Someone got tabs in the YAML!")
	}
	configTemplate = template.Must(template.New("cc").Parse(cloudConfig))
}

func BuildCloudConfig(dst io.Writer, ip, etcdEndpoint, etcdKeyPrefix string) error {
	key, err := ioutil.ReadFile(publicKeyPath)
	if err != nil {
		return err
	}

	values := configValues{
		IP:            ip,
		PublicKey:     string(key),
		EtcdEndpoint:  etcdEndpoint,
		EtcdKeyPrefix: etcdKeyPrefix,
		FleetAPIPort:  fleetAPIPort,
	}

	return configTemplate.Execute(dst, &values)
}
