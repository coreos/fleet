// Copyright 2016 CoreOS, Inc.
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

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/schema"
)

func checkLoadUnitState(unit schema.Unit, loadRet int, errchan chan error) {
	if loadRet == 0 {
		if job.JobState(unit.DesiredState) != job.JobStateLoaded {
			errchan <- fmt.Errorf("Error: unit %s was not loaded as requested", unit.Name)
		}
	} else if unit.DesiredState != "" {
		// if the whole load operation failed, then no unit
		// should have a DesiredState set
		errchan <- fmt.Errorf("Error: Unit(%s) DesiredState was set to (%s)", unit.Name, unit.DesiredState)
	}
}

func doLoadUnits(r commandTestResults, errchan chan error) {
	exit := runLoadUnits(r.units)
	if exit != r.expectedExit {
		errchan <- fmt.Errorf("%s: expected exit code %d but received %d", r.description, r.expectedExit, exit)
		return
	}

	real_units, err := findUnits(r.units)
	if err != nil {
		errchan <- err
		return
	}

	for _, v := range real_units {
		checkLoadUnitState(v, r.expectedExit, errchan)
	}
}

func TestRunLoadUnits(t *testing.T) {
	unitPrefix := "load"
	oldNoBlock := sharedFlags.NoBlock
	defer func() {
		sharedFlags.NoBlock = oldNoBlock
	}()

	results := []commandTestResults{
		{
			"load available units",
			[]string{"load1", "load2", "load3", "load4", "load5"},
			0,
		},
		{
			"load non-available units",
			[]string{"y1", "y2"},
			1,
		},
		{
			"load available and non-available units",
			[]string{"y1", "y2", "y3", "y4", "load1", "load2", "load3", "load4", "load5", "load6", "y0"},
			1,
		},
		{
			"load null input",
			[]string{},
			0,
		},
	}

	sharedFlags.NoBlock = true
	for _, r := range results {
		var wg sync.WaitGroup
		errchan := make(chan error)

		cAPI = newFakeRegistryForCommands(unitPrefix, len(r.units), false)

		wg.Add(2)
		go func() {
			defer wg.Done()
			doLoadUnits(r, errchan)
		}()
		go func() {
			defer wg.Done()
			doLoadUnits(r, errchan)
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
