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

	"github.com/coreos/fleet/client"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/unit"
)

func newFakeRegistryForDestroy() client.API {
	// clear machineStates for every invocation
	machineStates = nil
	machines := []machine.MachineState{
		newMachineState("c31e44e1-f858-436e-933e-59c642517860", "1.2.3.4", map[string]string{"ping": "pong"}),
		newMachineState("595989bb-cbb7-49ce-8726-722d6e157b4e", "5.6.7.8", map[string]string{"foo": "bar"}),
	}

	jobs := []job.Job{
		job.Job{Name: "j1.service", Unit: unit.UnitFile{}, TargetMachineID: machines[0].ID},
		job.Job{Name: "j2.service", Unit: unit.UnitFile{}, TargetMachineID: machines[1].ID},
	}

	states := []unit.UnitState{
		unit.UnitState{
			UnitName:    "j1.service",
			LoadState:   "loaded",
			ActiveState: "active",
			SubState:    "listening",
			MachineID:   machines[0].ID,
		},
		unit.UnitState{
			UnitName:    "j2.service",
			LoadState:   "loaded",
			ActiveState: "inactive",
			SubState:    "dead",
			MachineID:   machines[1].ID,
		},
	}

	reg := registry.NewFakeRegistry()
	reg.SetMachines(machines)
	reg.SetUnitStates(states)
	reg.SetJobs(jobs)

	return &client.RegistryClient{Registry: reg}
}

// TestRunDestroyUnits checks for correct unit destruction
func TestRunDestroyUnits(t *testing.T) {
	for _, s := range []struct {
		Description  string
		DestroyUnits []string
		ExpectedExit int
	}{
		{
			"destroy available units",
			[]string{"j1", "j2"},
			0,
		},
		{
			"attempt to destroy available and non-available units",
			[]string{"j1", "j2", "j3"},
			0,
		},
	} {
		cAPI = newFakeRegistryForDestroy()
		exit := runDestroyUnits(s.DestroyUnits)
		if exit != s.ExpectedExit {
			t.Errorf("%s: expected exit code %d but received %d",
				s.Description, s.ExpectedExit, exit)
		}
		for _, destroyedUnit := range s.DestroyUnits {
			u, _ := cAPI.Unit(destroyedUnit)
			if u != nil {
				t.Errorf("%s: unit %s was not destroyed as requested",
					s.Description, destroyedUnit)
			}
		}
	}
}
