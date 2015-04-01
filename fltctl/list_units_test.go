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

	"github.com/coreos/flt/job"
	"github.com/coreos/flt/machine"
	"github.com/coreos/flt/registry"
	"github.com/coreos/flt/schema"
)

func newFakeRegistryForListUnits(t *testing.T, jobs []job.Job) registry.Registry {
	reg := registry.NewFakeRegistry()
	reg.SetJobs(jobs)
	return reg
}

func assertEqual(t *testing.T, name string, want, got interface{}) {
	if want != got {
		t.Errorf("expected %q to be %q, got %q", name, want, got)
	}
}

func TestListUnitsFieldsToStrings(t *testing.T) {
	// nil UnitState shouldn't happen, but just in case
	for _, tt := range []string{"unit", "load", "active", "sub", "machine", "hash"} {
		f := listUnitsFields[tt](nil, false)
		assertEqual(t, tt, "-", f)
	}

	us := &schema.UnitState{
		Name:               "sleep",
		SystemdLoadState:   "foo",
		SystemdActiveState: "bar",
		SystemdSubState:    "baz",
		MachineID:          "",
	}

	for k, want := range map[string]string{
		"load":    "foo",
		"active":  "bar",
		"sub":     "baz",
		"machine": "-",
		"unit":    "sleep",
	} {
		got := listUnitsFields[k](us, false)
		assertEqual(t, k, want, got)
	}

	us.MachineID = "some-id"
	ms := listUnitsFields["machine"](us, true)
	assertEqual(t, "machine", "some-id", ms)

	us.MachineID = "other-id"
	machineStates = map[string]*machine.MachineState{
		"other-id": &machine.MachineState{
			ID:       "other-id",
			PublicIP: "1.2.3.4",
		},
	}
	ms = listUnitsFields["machine"](us, true)
	assertEqual(t, "machine", "other-id/1.2.3.4", ms)

	uh := "a0f275d46bc6ee0eca06be7c339913c07d99c0c7"
	us.Hash = uh
	fuh := listUnitsFields["hash"](us, true)
	suh := listUnitsFields["hash"](us, false)
	assertEqual(t, "hash", uh, fuh)
	assertEqual(t, "hash", uh[:7], suh)
}
