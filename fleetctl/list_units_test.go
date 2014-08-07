package main

import (
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/unit"
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

	us := unit.NewUnitState("foo", "bar", "baz", "")
	us.UnitName = "sleep"
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

	uh := "f035b2f14edc4d23572e5f3d3d4cb4f78d0e53c3"
	us.UnitHash = uh
	fuh := listUnitsFields["hash"](us, true)
	suh := listUnitsFields["hash"](us, false)
	assertEqual(t, "hash", uh, fuh)
	assertEqual(t, "hash", uh[:7], suh)
}
