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
	"strings"
	"sync"
	"testing"
)

func doSubmitUnits(r commandTestResults, errchan chan error) {
	exit := runSubmitUnits(r.units)
	if exit != r.expectedExit {
		errchan <- fmt.Errorf("%s: expected exit code %d but received %d", r.description, r.expectedExit, exit)
		return
	}

	submitted, err := findUnits(r.units)
	if err != nil {
		errchan <- err
		return
	}

	// In case an error is expected. don't check for existence of units, but just return
	if r.expectedExit != 0 {
		return
	}

	var found bool
	for _, inputUnit := range r.units {
		found = false

		for _, sUnit := range submitted {
			// sUnit.Name could contain a suffix ".service" like "foo1.service"
			if inputUnit == strings.TrimSuffix(sUnit.Name, ".service") {
				found = true
			}
		}

		if !found {
			errchan <- fmt.Errorf("%s: unit %s not found", r.description, inputUnit)
			return
		}
	}
}

func runSubmitUnitsTests(t *testing.T, unitPrefix string, results []commandTestResults, template bool) {
	unitsCount := 0
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
			doSubmitUnits(r, errchan)
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

func TestRunSubmitUnits(t *testing.T) {
	unitPrefix := "submit"
	oldNoBlock := sharedFlags.NoBlock
	defer func() {
		sharedFlags.NoBlock = oldNoBlock
	}()

	results := []commandTestResults{
		{
			"submit available units",
			[]string{"submit1", "submit2", "submit3", "submit4", "submit5"},
			0,
		},
		{
			"submit non-available units",
			[]string{"y1", "y2"},
			1,
		},
		{
			"submit available and non-available units",
			[]string{"y1", "y2", "y3", "y4", "submit1", "submit2", "submit3", "submit4", "submit5", "submit6", "y0"},
			1,
		},
		{
			"submit same unit multiple times",
			[]string{"submit1", "submit1", "submit1"},
			0,
		},
		{
			"submit same unit with non-available units",
			[]string{"submit1", "submit1", "submit1", "y0"},
			1,
		},
		{
			"submit null input",
			[]string{},
			0,
		},
	}

	templateResults := []commandTestResults{
		{
			"submit a unit from a non-available template",
			[]string{"submit-foo@1"},
			1,
		},
		{
			"submit units from an available template",
			[]string{"submit@1", "submit@100", "submit@1000"},
			0,
		},
		{
			"submit units from available and non-available templates",
			[]string{"y@1", "y@2", "y@3", "y@4", "submit@1", "submit@2", "submit@3", "submit@4", "submit@5", "submit@6", "y@0"},
			1,
		},
		{
			"submit same unit from an available template",
			[]string{"submit@1", "submit@1", "submit@1"},
			0,
		},
		{
			"submit same unit from an available template with units from non-available templates",
			[]string{"submit@1", "submit@1", "submit@1", "y@0"},
			1,
		},
	}

	sharedFlags.NoBlock = true
	runSubmitUnitsTests(t, unitPrefix, results, false)
	runSubmitUnitsTests(t, unitPrefix, templateResults, true)
}
