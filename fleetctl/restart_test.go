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

	"github.com/coreos/fleet/schema"
)

func checkRestartUnitState(unit schema.Unit, restartRet int, errchan chan error) {
	if restartRet != 0 && unit.DesiredState != "" {
		// if the whole restart operation failed, then no unit
		// should have a DesiredState set
		errchan <- fmt.Errorf("Error: Unit(%s) DesiredState was set to (%s)", unit.Name, unit.DesiredState)
	}
}

func doRestartUnits(t *testing.T, r commandTestResults, errchan chan error) {
	sharedFlags.NoBlock = true
	exit := runRestartUnit(cmdRestart, r.units)
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
		checkRestartUnitState(v, r.expectedExit, errchan)
	}
}

func runRestartUnits(t *testing.T, unitPrefix string, results []commandTestResults, template bool) {
	unitsCount := 0
	for _, r := range results {
		var wg_res sync.WaitGroup

		if !template {
			unitsCount = len(r.units)
		}

		cAPI = newFakeRegistryForCommands(unitPrefix, unitsCount, template)

		errchan_res := make(chan error)
		wg_res.Add(1)
		go func() {
			defer wg_res.Done()
			doRestartUnits(t, r, errchan_res)
		}()

		go func() {
			wg_res.Wait()
			close(errchan_res)
		}()

		for err := range errchan_res {
			t.Errorf("%v", err)
		}
	}
}

func TestRunRestartUnits(t *testing.T) {
	unitPrefix := "restart"
	oldNoBlock := sharedFlags.NoBlock
	defer func() {
		sharedFlags.NoBlock = oldNoBlock
	}()

	results := []commandTestResults{
		{
			"restart available units",
			[]string{"restart1", "restart2", "restart3", "restart4", "restart5", "restart6"},
			0,
		},
		{
			"restart non-available units",
			[]string{"y1", "y2"},
			1,
		},
		{
			"restart available and non-available units",
			[]string{"y1", "y2", "y3", "y4", "restart1", "restart2", "restart3", "restart4", "restart5", "restart6", "y0"},
			1,
		},
		{
			"restart a unit from a non-available template",
			[]string{"foo-template@1"},
			1,
		},
		{
			"restart null input",
			[]string{},
			0,
		},
	}

	templateResults := []commandTestResults{
		{
			"restart a unit from a non-available template",
			[]string{"restart-foo@1"},
			1,
		},
		{
			"restart units from an available template",
			[]string{"restart@1", "restart@100", "restart@1000"},
			0,
		},
		{
			"restart same unit from an available template",
			[]string{"restart@1", "restart@1", "restart@1"},
			0,
		},
	}

	runRestartUnits(t, unitPrefix, results, false)
	runRestartUnits(t, unitPrefix, templateResults, true)
}
