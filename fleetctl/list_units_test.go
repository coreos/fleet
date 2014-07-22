package main

import (
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/unit"
)

func newNamedTestJobFromUnitContents(t *testing.T, name, contents string) *job.Job {
	u, err := unit.NewUnit(contents)
	if err != nil {
		t.Fatalf("error creating Unit from %q: %v", contents, err)
	}
	j := job.NewJob(name, *u)
	if j == nil {
		t.Fatalf("error creating Job %q from %q", name, u)
	}
	return j
}

func newTestJobFromUnitContents(t *testing.T, contents string) *job.Job {
	return newNamedTestJobFromUnitContents(t, "foo.service", contents)
}

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
	j := newTestJobFromUnitContents(t, "")
	for _, tt := range []string{"state", "load", "active", "sub", "desc", "machine"} {
		f := listUnitsFields[tt](j, false)
		assertEqual(t, tt, "-", f)
	}

	f := listUnitsFields["unit"](j, false)
	assertEqual(t, "unit", j.Name, f)

	j = newTestJobFromUnitContents(t, `[Unit]
Description=some description`)
	d := listUnitsFields["desc"](j, false)
	assertEqual(t, "desc", "some description", d)

	for _, state := range []job.JobState{job.JobStateLoaded, job.JobStateInactive, job.JobStateLaunched} {
		j.State = &state
		f := listUnitsFields["state"](j, false)
		assertEqual(t, "state", string(state), f)
	}

	j.UnitState = unit.NewUnitState("foo", "bar", "baz", "")
	for k, want := range map[string]string{
		"load":    "foo",
		"active":  "bar",
		"sub":     "baz",
		"machine": "-",
	} {
		got := listUnitsFields[k](j, false)
		assertEqual(t, k, want, got)
	}

	j.UnitState.MachineID = "some-id"
	ms := listUnitsFields["machine"](j, true)
	assertEqual(t, "machine", "some-id", ms)

	j.UnitState.MachineID = "other-id"
	machineStates = map[string]*machine.MachineState{
		"other-id": &machine.MachineState{
			ID:       "other-id",
			PublicIP: "1.2.3.4",
		},
	}
	ms = listUnitsFields["machine"](j, true)
	assertEqual(t, "machine", "other-id/1.2.3.4", ms)

	uh := "f035b2f14edc4d23572e5f3d3d4cb4f78d0e53c3"
	j.UnitState.UnitHash = uh
	fuh := listUnitsFields["hash"](j, true)
	suh := listUnitsFields["hash"](j, false)
	assertEqual(t, "hash", uh, fuh)
	assertEqual(t, "hash", uh[:7], suh)
}
