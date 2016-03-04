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

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/schema"
)

func checkStartUnitState(unit schema.Unit, startRet int, errchan chan error) {
	if startRet == 0 {
		if job.JobState(unit.DesiredState) != job.JobStateLaunched {
			errchan <- fmt.Errorf("Error: unit %s was not started as requested", unit.Name)
		}
	} else if unit.DesiredState != "" {
		// if the whole start operation failed, then no unit
		// should have a DesiredState set
		errchan <- fmt.Errorf("Error: Unit(%s) DesiredState was set to (%s)", unit.Name, unit.DesiredState)
	}
}

func doStartUnits(r commandTestResults, errchan chan error) {
	exit := runStartUnit(r.units)
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
		checkStartUnitState(v, r.expectedExit, errchan)
	}
}

func runStartUnits(t *testing.T, unitPrefix string, results []commandTestResults, template bool) {
	unitsCount := 0
	sharedFlags.NoBlock = true
	for _, r := range results {
		var wg sync.WaitGroup
		errchan := make(chan error)

		if !template {
			unitsCount = len(r.units)
		}

		cAPI = newFakeRegistryForCommands(unitPrefix, unitsCount, template)

		wg.Add(1)
		go func() {
			defer wg.Done()
			doStartUnits(r, errchan)
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

func TestRunStartUnits(t *testing.T) {
	unitPrefix := "start"
	oldNoBlock := sharedFlags.NoBlock
	defer func() {
		sharedFlags.NoBlock = oldNoBlock
	}()

	results := []commandTestResults{
		{
			"start available units",
			[]string{"start1", "start2", "start3", "start4", "start5", "start6"},
			0,
		},
		{
			"start non-available units",
			[]string{"y1", "y2"},
			1,
		},
		{
			"start available and non-available units",
			[]string{"y1", "y2", "y3", "y4", "start1", "start2", "start3", "start4", "start5", "start6", "y0"},
			1,
		},
		{
			"start a unit from a non-available template",
			[]string{"foo-template@1"},
			1,
		},
	}

	templateResults := []commandTestResults{
		{
			"start a unit from a non-available template",
			[]string{"start-foo@1"},
			1,
		},
		{
			"start units from an available template",
			[]string{"start@1", "start@100", "start@1000"},
			0,
		},
		{
			"start same unit from an available template",
			[]string{"start@1", "start@1", "start@1"},
			0,
		},
	}

	sharedFlags.NoBlock = true
	runStartUnits(t, unitPrefix, results, false)
	runStartUnits(t, unitPrefix, templateResults, true)
}
