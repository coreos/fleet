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

package main

import (
	"testing"

	"github.com/coreos/flt/machine"
	"github.com/coreos/flt/registry"
)

func newTestRegistryForListMachines() registry.Registry {
	m := []machine.MachineState{
		machine.MachineState{ID: "mnopqr"},
		machine.MachineState{ID: "abcdef"},
		machine.MachineState{ID: "ghijkl"},
	}

	reg := registry.NewFakeRegistry()
	reg.SetMachines(m)

	return reg
}

func TestListMachinesFieldsToStrings(t *testing.T) {
	id := "4d389537d9d14bdabe8be54a9c29f68d"
	ip := "192.0.2.1"
	metadata := map[string]string{
		"foo":  "bar",
		"ping": "pong",
	}
	ver := "v9.9.9"

	ms := &machine.MachineState{
		ID:       id,
		PublicIP: ip,
		Metadata: metadata,
		Version:  ver,
	}

	val := listMachinesFields["machine"](ms, false)
	assertEqual(t, "machine", "4d389537...", val)

	val = listMachinesFields["machine"](ms, true)
	assertEqual(t, "machine", "4d389537d9d14bdabe8be54a9c29f68d", val)

	val = listMachinesFields["ip"](ms, false)
	assertEqual(t, "ip", "192.0.2.1", val)

	val = listMachinesFields["metadata"](ms, false)
	assertEqual(t, "metadata", "foo=bar,ping=pong", val)
}

func TestListMachinesFieldsEmpty(t *testing.T) {
	id := "4d389537d9d14bdabe8be54a9c29f68d"
	ip := ""
	metadata := map[string]string{}
	ver := "v9.9.9"

	ms := &machine.MachineState{
		ID:       id,
		PublicIP: ip,
		Metadata: metadata,
		Version:  ver,
	}

	for _, tt := range []string{"ip", "metadata"} {
		f := listMachinesFields[tt](ms, false)
		assertEqual(t, tt, "-", f)
	}
}
