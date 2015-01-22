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
	"time"

	"github.com/coreos/fleet/client"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/unit"
)

type BlockedFakeRegistry struct {
	EchoAttempts int
	registry.FakeRegistry
}

func (b *BlockedFakeRegistry) Unit(name string) (*job.Unit, error) {
	if name == "hello.service" {
		time.Sleep(500 * time.Millisecond)
	}

	if name == "echo.service" {
		if b.EchoAttempts != 0 {
			b.EchoAttempts--
			return nil, nil
		}
	}

	return b.FakeRegistry.Unit(name)
}

func setupRegistryForStart(echoAttempts int) {
	m1 := machine.MachineState{
		ID:       "c31e44e1-f858-436e-933e-59c642517860",
		PublicIP: "1.2.3.4",
		Metadata: map[string]string{"ping": "pong"},
	}
	m2 := machine.MachineState{
		ID:       "595989bb-cbb7-49ce-8726-722d6e157b4e",
		PublicIP: "5.6.7.8",
		Metadata: map[string]string{"foo": "bar"},
	}
	m3 := machine.MachineState{
		ID:       "520983A8-FB9C-4A68-B49C-CED5BB2E9D08",
		Metadata: map[string]string{"foo": "bar"},
	}

	states := []unit.UnitState{
		unit.UnitState{
			UnitName:    "pong.service",
			LoadState:   "loaded",
			ActiveState: "active",
			SubState:    "listening",
			MachineID:   m1.ID,
		},
		unit.UnitState{
			UnitName:    "hello.service",
			LoadState:   "loaded",
			ActiveState: "inactive",
			SubState:    "dead",
			MachineID:   m2.ID,
		},
		unit.UnitState{
			UnitName:    "echo.service",
			LoadState:   "loaded",
			ActiveState: "inactive",
			SubState:    "dead",
			MachineID:   m2.ID,
		},
	}

	machines := []machine.MachineState{m1, m2, m3}

	reg := registry.NewFakeRegistry()
	reg.SetMachines(machines)
	reg.SetUnitStates(states)

	cAPI = &client.RegistryClient{Registry: &BlockedFakeRegistry{EchoAttempts: echoAttempts, FakeRegistry: *reg}}
}
