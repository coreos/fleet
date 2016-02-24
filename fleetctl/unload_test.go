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
)

func doUnloadUnits(r commandTestResults, errchan chan error) {
	exit := runUnloadUnit(r.units)
	if exit != r.expectedExit {
		errchan <- fmt.Errorf("%s: expected exit code %d but received %d", r.description, r.expectedExit, exit)
	}

	real_units, err := findUnits(r.units)
	if err != nil {
		errchan <- err
		return
	}

	// We assume that we reached the desired state
	for _, v := range real_units {
		if job.JobState(v.DesiredState) != job.JobStateInactive {
			errchan <- fmt.Errorf("Error: unit %s was not unloaded as requested", v.Name)
		}
	}
}

func TestRunUnloadUnits(t *testing.T) {
	unitPrefix := "unload"
	oldNoBlock := sharedFlags.NoBlock
	defer func() {
		sharedFlags.NoBlock = oldNoBlock
	}()

	results := []commandTestResults{
		{
			"unload available units",
			[]string{"unload1", "unload2", "unload3", "unload4", "unload5"},
			0,
		},
		{
			"unload non-available units",
			[]string{"y1", "y2"},
			0,
		},
		{
			"attempt to unload available and non-available units",
			[]string{"y1", "y2", "y3", "y4", "unload1", "unload2", "unload3", "unload4", "unload5", "y0"},
			0,
		},
	}

	sharedFlags.NoBlock = true
	for _, r := range results {
		var wg sync.WaitGroup
		errchan := make(chan error)

		cAPI = newFakeRegistryForCommands(unitPrefix, len(r.units))

		wg.Add(2)
		go func() {
			defer wg.Done()
			doUnloadUnits(r, errchan)
		}()
		go func() {
			defer wg.Done()
			doUnloadUnits(r, errchan)
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
