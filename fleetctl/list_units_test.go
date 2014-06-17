package main

import (
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/resource"
	"github.com/coreos/fleet/unit"
)

func newFakeRegistryForListUnits(jobs []job.Job) registry.Registry {
	j := []job.Job{*job.NewJob("pong.service", *unit.NewUnit("Echo"))}

	if jobs != nil {
		for _, job := range jobs {
			j = append(j, job)
		}
	}

	reg := registry.NewFakeRegistry()
	reg.SetJobs(j)

	return reg
}

func TestGetAllJobs(t *testing.T) {
	fc = newFakeRegistryForListUnits(nil)

	jobs, sortable, err := findAllUnits()
	if err != nil {
		t.Fatalf("Unexpected error getting all units: %v\n", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("Expected to find one unit: %v\n", jobs)
	}

	if sortable[0] != "pong.service" {
		t.Errorf("Expected to find pong.service as the first name, but it was %s\n", sortable[0])
	}
}

func TestJobDescription(t *testing.T) {
	contents := `[Unit]
Description=PING
`
	j := []job.Job{*job.NewJob("ping.service", *unit.NewUnit(contents))}
	fc = newFakeRegistryForListUnits(j)

	jobs, _, _ := findAllUnits()
	if len(jobs) != 2 {
		t.Errorf("Expected to find two units: %v\n", jobs)
	}

	ping := jobs["ping.service"]
	desc := ping.Unit.Description()
	if desc != "PING" {
		t.Errorf("Expected to have `PING` as a description, but it was %s\n", desc)
	}
}

func assertEqual(t *testing.T, name string, want, got interface{}) {
	if want != got {
		t.Errorf("expected %q to be %q, got %q", name, want, got)
	}
}

func TestListUnitsFieldsToStrings(t *testing.T) {
	j := job.NewJob("test", *unit.NewUnit(""))
	for _, tt := range []string{"state", "load", "active", "sub", "desc", "machine"} {
		f := listUnitsFields[tt](j, false)
		assertEqual(t, tt, "-", f)
	}

	f := listUnitsFields["unit"](j, false)
	assertEqual(t, "unit", "test", f)

	j = job.NewJob("test", *unit.NewUnit(`[Unit]
Description=some description`))
	d := listUnitsFields["desc"](j, false)
	assertEqual(t, "desc", "some description", d)

	for _, state := range []job.JobState{job.JobStateLoaded, job.JobStateInactive, job.JobStateLaunched} {
		j.State = &state
		f := listUnitsFields["state"](j, false)
		assertEqual(t, "state", string(state), f)
	}

	j.UnitState = unit.NewUnitState("foo", "bar", "baz", nil)
	for k, want := range map[string]string{
		"load":    "foo",
		"active":  "bar",
		"sub":     "baz",
		"machine": "-",
	} {
		got := listUnitsFields[k](j, false)
		assertEqual(t, k, want, got)
	}

	j.UnitState.MachineState = &machine.MachineState{"some-id", "1.2.3.4", nil, "", resource.ResourceTuple{}}
	ms := listUnitsFields["machine"](j, true)
	assertEqual(t, "machine", "some-id/1.2.3.4", ms)

	uh := "f035b2f14edc4d23572e5f3d3d4cb4f78d0e53c3"
	fuh := listUnitsFields["hash"](j, true)
	suh := listUnitsFields["hash"](j, false)
	assertEqual(t, "hash", uh, fuh)
	assertEqual(t, "hash", uh[:7], suh)
}
