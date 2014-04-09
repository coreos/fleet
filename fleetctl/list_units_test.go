package main

import (
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/unit"
)

func newTestRegistryForListUnits(payloads []job.JobPayload, jobs []job.Job) Registry {
	j := []job.Job{*job.NewJob("pong.service", map[string][]string{}, nil)}
	p := []job.JobPayload{*job.NewJobPayload("echo.service", *unit.NewSystemdUnitFile("Echo"))}

	if payloads != nil {
		for _, jp := range payloads {
			p = append(p, jp)
		}
	}

	if jobs != nil {
		for _, job := range jobs {
			j = append(j, job)
		}
	}

	return TestRegistry{jobs: j, payloads: p}
}

func TestGetAllJobs(t *testing.T) {
	registryCtl = newTestRegistryForListUnits(nil, nil)

	names, sortable := findAllUnits()
	if len(names) != 2 {
		t.Errorf("Expected to find two units: %v\n", names)
	}

	if sortable[0] != "echo.service" {
		t.Errorf("Expected to find echo.service as the first name, but it was %s\n", sortable[0])
	}

	if sortable[1] != "pong.service" {
		t.Errorf("Expected to find pong.service as the second name, but it was %s\n", sortable[0])
	}
}

func TestIgnoreDuplicatedUnits(t *testing.T) {
	jp := []job.JobPayload{*job.NewJobPayload("echo.service", *unit.NewSystemdUnitFile("Echo"))}
	registryCtl = newTestRegistryForListUnits(jp, nil)

	names, sortable := findAllUnits()
	if len(names) != 2 {
		t.Errorf("Expected to find two units: %v\n", names)
	}

	if sortable[0] != "echo.service" {
		t.Errorf("Expected to find echo.service as the first name, but it was %s\n", sortable[0])
	}

	if sortable[1] != "pong.service" {
		t.Errorf("Expected to find pong.service as the second name, but it was %s\n", sortable[0])
	}
}

func TestJobDescription(t *testing.T) {
	contents := `[Unit]
Description=PING
`
	jp := job.NewJobPayload("ping.service", *unit.NewSystemdUnitFile(contents))
	j := []job.Job{*job.NewJob("ping.service", map[string][]string{}, jp)}
	registryCtl = newTestRegistryForListUnits(nil, j)

	names, _ := findAllUnits()
	if len(names) != 3 {
		t.Errorf("Expected to find three units: %v\n", names)
	}

	if names["ping.service"] != "PING" {
		t.Errorf("Expected to have `PING` as a description, but it was %s\n", names["ping.service"])
	}
}

func TestPayloadDescription(t *testing.T) {
	contents := `[Unit]
Description=PING
`
	jp := []job.JobPayload{*job.NewJobPayload("ping.service", *unit.NewSystemdUnitFile(contents))}
	registryCtl = newTestRegistryForListUnits(jp, nil)

	names, _ := findAllUnits()
	if len(names) != 3 {
		t.Errorf("Expected to find three units: %v\n", names)
	}

	if names["ping.service"] != "PING" {
		t.Errorf("Expected to have `PING` as a description, but it was %s\n", names["ping.service"])
	}
}
