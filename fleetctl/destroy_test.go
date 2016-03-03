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
)

func doDestroyUnits(r commandTestResults, errchan chan error) {
	exit := runDestroyUnits(r.units)
	if exit != r.expectedExit {
		errchan <- fmt.Errorf("%s: expected exit code %d but received %d", r.description, r.expectedExit, exit)
		return
	}
	for _, destroyedUnit := range r.units {
		u, _ := cAPI.Unit(destroyedUnit)
		if u != nil {
			errchan <- fmt.Errorf("%s: unit %s was not destroyed as requested", r.description, destroyedUnit)
		}
	}
}

// TestRunDestroyUnits checks for correct unit destruction
func TestRunDestroyUnits(t *testing.T) {
	unitPrefix := "j"
	results := []commandTestResults{
		{
			"destroy available units",
			[]string{"j1", "j2", "j3", "j4", "j5"},
			0,
		},
		{
			"destroy non-available units",
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

		cAPI = newFakeRegistryForCommands(unitPrefix, len(r.units), false)

		wg.Add(2)
		go func() {
			defer wg.Done()
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
