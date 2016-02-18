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
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/coreos/fleet/client"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/unit"
)

type DestroyTestResults struct {
	Description  string
	DestroyUnits []string
	ExpectedExit int
}

func newFakeRegistryForDestroy(prefix string, unitCnt int) client.API {
	// clear machineStates for every invocation
	machineStates = nil
	machines := []machine.MachineState{
		newMachineState("c31e44e1-f858-436e-933e-59c642517860", "1.2.3.4", map[string]string{"ping": "pong"}),
		newMachineState("595989bb-cbb7-49ce-8726-722d6e157b4e", "5.6.7.8", map[string]string{"foo": "bar"}),
	}

	jobs := make([]job.Job, 0)
	appendJobsForTests(&jobs, machines[0], prefix, unitCnt)
	appendJobsForTests(&jobs, machines[1], prefix, unitCnt)

	states := make([]unit.UnitState, 0)
	for i := 1; i <= unitCnt; i++ {
		state := unit.UnitState{
			UnitName:    fmt.Sprintf("%s%d.service", prefix, i),
			LoadState:   "loaded",
			ActiveState: "active",
			SubState:    "listening",
			MachineID:   machines[0].ID,
		}
		states = append(states, state)
	}

	for i := 1; i <= unitCnt; i++ {
		state := unit.UnitState{
			UnitName:    fmt.Sprintf("%s%d.service", prefix, i),
			LoadState:   "loaded",
			ActiveState: "inactive",
			SubState:    "dead",
			MachineID:   machines[1].ID,
		}
		states = append(states, state)
	}

	reg := registry.NewFakeRegistry()
	reg.SetMachines(machines)
	reg.SetUnitStates(states)
	reg.SetJobs(jobs)

	return &client.RegistryClient{Registry: reg}
}

func doDestroyUnits(r DestroyTestResults, errchan chan error) {
	exit := runDestroyUnits(r.DestroyUnits)
	if exit != r.ExpectedExit {
		errchan <- fmt.Errorf("%s: expected exit code %d but received %d", r.Description, r.ExpectedExit, exit)
	}
	for _, destroyedUnit := range r.DestroyUnits {
		u, _ := cAPI.Unit(destroyedUnit)
		if u != nil {
			errchan <- fmt.Errorf("%s: unit %s was not destroyed as requested", r.Description, destroyedUnit)
		}
	}
}

// TestRunDestroyUnits checks for correct unit destruction
func TestRunDestroyUnits(t *testing.T) {
	unitPrefix := "j"
	results := []DestroyTestResults{
		{
			"destroy available units",
			[]string{"j1", "j2", "j3", "j4", "j5"},
			0,
		},
		{
			"destroy non-existent units",
			[]string{"y1", "y2"},
			0,
		},
		{
			"attempt to destroy available and non-available units",
			[]string{"y1", "y2", "y3", "y4", "j1", "j2", "j3", "j4", "j5", "y0"},
			0,
		},
	}

	// Check with two goroutines we don't care we should just get
	// the right result. If you happen to inspect this code for
	// errors then you probably got hit by a race condition in
	// Destroy command that should not happen
	for _, r := range results {
		var wg sync.WaitGroup
		errchan := make(chan error)

		cAPI = newFakeRegistryForDestroy(unitPrefix, len(r.DestroyUnits))

		wg.Add(2)
		go func() {
			defer wg.Done()
			time.Sleep(2 * time.Microsecond)
			doDestroyUnits(r, errchan)
		}()
		go func() {
			defer wg.Done()
			doDestroyUnits(r, errchan)
		}()

		go func() {
			wg.Wait()
			close(errchan)
		}()

		for err := range errchan {
			t.Errorf("%v", err)
		}
	}
}
