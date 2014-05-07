package main

import (
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/unit"
)

func newTestRegistryForListUnits(jobs []job.Job) registry.Registry {
	j := []job.Job{*job.NewJob("pong.service", *unit.NewUnit("Echo"))}

	if jobs != nil {
		for _, job := range jobs {
			j = append(j, job)
		}
	}

	return TestRegistry{jobs: j}
}

func TestGetAllJobs(t *testing.T) {
	registryCtl = newTestRegistryForListUnits(nil)

	jobs, sortable := findAllUnits()
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

	jobs, _ := findAllUnits()
	if len(jobs) != 2 {
		t.Errorf("Expected to find two units: %v\n", jobs)
	}

	ping := jobs["ping.service"]
	desc := ping.Unit.Description()
	if desc != "PING" {
		t.Errorf("Expected to have `PING` as a description, but it was %s\n", desc)
	}
}
