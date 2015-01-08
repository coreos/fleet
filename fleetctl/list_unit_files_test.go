/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package main

import (
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/schema"
)

func TestListUnitFilesFieldsToStrings(t *testing.T) {
	u := schema.Unit{
		Name:    "foo.service",
		Options: []*schema.UnitOption{},
	}

	for k, v := range map[string]string{
		"hash":     "da39a3e",
		"desc":     "-",
		"dstate":   "-",
		"tmachine": "-",
		"state":    "-",
	} {
		f := listUnitFilesFields[k](u, false)
		assertEqual(t, k, v, f)
	}

	f := listUnitFilesFields["unit"](u, false)
	assertEqual(t, "unit", u.Name, f)

	u = schema.Unit{
		Name: "foo.service",
		Options: []*schema.UnitOption{
			&schema.UnitOption{Section: "Unit", Name: "Description", Value: "some description"},
		},
	}

	d := listUnitFilesFields["desc"](u, false)
	assertEqual(t, "desc", "some description", d)

	for _, state := range []job.JobState{job.JobStateLoaded, job.JobStateInactive, job.JobStateLaunched} {
		u.CurrentState = string(state)
		f := listUnitFilesFields["state"](u, false)
		assertEqual(t, "state", string(state), f)
	}

	// machineStates must be initialized since cAPI is not set
	machineStates = map[string]*machine.MachineState{}

	u.MachineID = "some-id"
	ms := listUnitFilesFields["tmachine"](u, true)
	assertEqual(t, "machine", "some-id", ms)

	u.MachineID = "other-id"
	machineStates = map[string]*machine.MachineState{
		"other-id": &machine.MachineState{
			ID:       "other-id",
			PublicIP: "1.2.3.4",
		},
	}
	ms = listUnitFilesFields["tmachine"](u, true)
	assertEqual(t, "machine", "other-id/1.2.3.4", ms)

	uh := "a0f275d46bc6ee0eca06be7c339913c07d99c0c7"
	fuh := listUnitFilesFields["hash"](u, true)
	suh := listUnitFilesFields["hash"](u, false)
	assertEqual(t, "hash", uh, fuh)
	assertEqual(t, "hash", uh[:7], suh)
}

func TestMapTargetField(t *testing.T) {
	// seeding the cache for the following test cases
	machineStates = map[string]*machine.MachineState{
		"XXX": &machine.MachineState{ID: "XXX"},
	}

	tests := []struct {
		unit schema.Unit
		want string
	}{
		// already scheduled
		{
			unit: schema.Unit{
				MachineID: "XXX",
			},
			want: "XXX",
		},
		// not yet scheduled
		{
			unit: schema.Unit{},
			want: "-",
		},
		// global unit
		{
			unit: schema.Unit{
				Options: []*schema.UnitOption{
					&schema.UnitOption{Section: "X-Fleet", Name: "Global", Value: "true"},
				},
			},
			want: "global",
		},
	}

	for i, tt := range tests {
		// eliminate the "full" variable from test cases by hard-coding "true" below
		got := mapTargetField(tt.unit, true)
		if tt.want != got {
			t.Errorf("case %d: want=%q got=%q", i, tt.want, got)
		}
	}
}
