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
	"bytes"
	"compress/gzip"
	"encoding/base64"
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
{{if .Fleetd}}
 - path: /opt/fleet/fleetd
   permissions: 0755
   encoding: gzip+base64
   content: {{.Fleetd}}
{{end}}
 - path: /opt/fleet/fleet.conf
   content: |
    verbosity=2
    etcd_servers=[{{printf "%q" .EtcdEndpoint}}]
    etcd_key_prefix={{.EtcdKeyPrefix}}

ssh_authorized_keys:
 - {{printf "%q" .PublicKey}}

coreos:
 units:
{{if .NetworkUnit}}
  - name: 00-local.network
    content: {{printf "%q" .NetworkUnit}}
{{end}}
  - name: fleet.socket
    command: start
    content: |
     [Socket]
     ListenStream={{printf "%d" .FleetAPIPort}}
  - name: fleet.service
    command: start
    content: |
     [Service]
     ExecStart=/opt/fleet/fleetd -config /opt/fleet/fleet.conf
`
)

var (
	fleetdBinPath  string
	publicKeyPath  string
	configTemplate *template.Template
)

type configValues struct {
	NetworkUnit   string
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

func encodeFile(path string) (string, error) {
	var dst bytes.Buffer
	b64 := base64.NewEncoder(base64.StdEncoding, &dst)
	gz := gzip.NewWriter(b64)

	src, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer src.Close()

	if _, err = io.Copy(gz, src); err != nil {
		return "", err
	}

	if err = gz.Close(); err != nil {
		return "", err
	}

	if err = b64.Close(); err != nil {
		return "", err
	}

	return dst.String(), nil
}

func BuildCloudConfig(dst io.Writer, networkUnit, etcdEndpoint, etcdKeyPrefix string, inlineFleetd bool) error {
	var fleetd string
	if inlineFleetd {
		var err error
		fleetd, err = encodeFile(fleetdBinPath)
		if err != nil {
			return err
		}
	}

	key, err := ioutil.ReadFile(publicKeyPath)
	if err != nil {
		return err
	}

	values := configValues{
		NetworkUnit:   networkUnit,
		PublicKey:     string(key),
		Fleetd:        fleetd,
		EtcdEndpoint:  etcdEndpoint,
		EtcdKeyPrefix: etcdKeyPrefix,
		FleetAPIPort:  fleetAPIPort,
	}

	if err = configTemplate.Execute(dst, &values); err != nil {
		return err
	}

	return nil
}
