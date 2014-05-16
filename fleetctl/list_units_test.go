package main

import (
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/resource"
	"github.com/coreos/fleet/unit"
)

func newTestRegistryForListUnits(jobs []job.Job) registry.Registry {
	j := []job.Job{*job.NewJob("pong.service", *unit.NewUnit("Echo"))}

	if jobs != nil {
		for _, job := range jobs {
			j = append(j, job)
		}
	}

	return registry.NewTestRegistry(nil, nil, j, nil, nil)
}

func TestGetAllJobs(t *testing.T) {
	registryCtl = newTestRegistryForListUnits(nil)

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
	registryCtl = newTestRegistryForListUnits(j)

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

func TestFieldsToStrings(t *testing.T) {
	j := job.NewJob("test", *unit.NewUnit(""))
	for _, tt := range []string{"state", "load", "active", "sub", "desc", "machine"} {
		f := fieldsToOutput[tt](j, false)
		assertEqual(t, tt, "-", f)
	}

	f := fieldsToOutput["unit"](j, false)
	assertEqual(t, "unit", "test", f)

	j = job.NewJob("test", *unit.NewUnit(`[Unit]
Description=some description`))
	d := fieldsToOutput["desc"](j, false)
	assertEqual(t, "desc", "some description", d)

	for _, state := range []job.JobState{job.JobStateLoaded, job.JobStateInactive, job.JobStateLaunched} {
		j.State = &state
		f := fieldsToOutput["state"](j, false)
		assertEqual(t, "state", string(state), f)
	}

	j.UnitState = unit.NewUnitState("foo", "bar", "baz", nil)
	for k, want := range map[string]string{
		"load":    "foo",
		"active":  "bar",
		"sub":     "baz",
		"machine": "-",
	} {
		got := fieldsToOutput[k](j, false)
		assertEqual(t, k, want, got)
	}

	j.UnitState.MachineState = &machine.MachineState{"some-id", "1.2.3.4", nil, "", resource.ResourceTuple{}}
	ms := fieldsToOutput["machine"](j, true)
	assertEqual(t, "machine", "some-id/1.2.3.4", ms)

	uh := "f035b2f14edc4d23572e5f3d3d4cb4f78d0e53c3"
	fuh := fieldsToOutput["hash"](j, true)
	suh := fieldsToOutput["hash"](j, false)
	assertEqual(t, "hash", uh, fuh)
	assertEqual(t, "hash", uh[:7], suh)
}
