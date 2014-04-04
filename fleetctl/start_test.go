package main

import (
	"bytes"
	"testing"
	"time"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
)

type BlockedTestRegistry struct {
	EchoAttempts int
	TestRegistry
}

func (b BlockedTestRegistry) GetJobTarget(name string) *machine.MachineState {
	if name == "hello.service" {
		time.Sleep(500 * time.Millisecond)
	}

	if name == "echo.service" {
		if b.EchoAttempts != 0 {
			b.EchoAttempts--
			return nil
		}
	}

	return b.TestRegistry.GetJobTarget(name)
}

func setupRegistryForStart(echoAttempts int) {
	m1 := machine.MachineState{"c31e44e1-f858-436e-933e-59c642517860", "1.2.3.4", map[string]string{"ping": "pong"}, ""}
	m2 := machine.MachineState{"595989bb-cbb7-49ce-8726-722d6e157b4e", "5.6.7.8", map[string]string{"foo": "bar"}, ""}
	m3 := machine.MachineState{"520983A8-FB9C-4A68-B49C-CED5BB2E9D08", "", map[string]string{"foo": "bar"}, ""}

	js := job.NewJobState("loaded", "active", "listening", []string{}, &m1)
	js2 := job.NewJobState("loaded", "inactive", "dead", []string{}, &m2)
	js3 := job.NewJobState("loaded", "inactive", "dead", []string{}, &m2)
	js4 := job.NewJobState("loaded", "inactive", "dead", []string{}, &m3)

	states := map[string]*job.JobState{"pong.service": js, "hello.service": js2, "echo.service": js3, "private.service": js4}
	machines := []machine.MachineState{m1, m2, m3}

	registryCtl = BlockedTestRegistry{echoAttempts, TestRegistry{jobStates: states, machines: machines}}
}

func TestPrintJobStarted(t *testing.T) {
	setupRegistryForStart(0)

	jobs := map[string]bool{"pong.service": true}
	b := bytes.NewBufferString("")
	waitForScheduledUnits(jobs, 1, b)

	if b.String() != "Job pong.service scheduled to c31e44e1.../1.2.3.4\n" {
		t.Errorf("Found unexpected start output: %s", b.String())
	}
}

func TestPrintSeveralJobs(t *testing.T) {
	setupRegistryForStart(0)

	jobs := map[string]bool{"hello.service": true, "pong.service": true}
	b := bytes.NewBufferString("")
	waitForScheduledUnits(jobs, 1, b)

	expected := `Job pong.service scheduled to c31e44e1.../1.2.3.4
Job hello.service scheduled to 595989bb.../5.6.7.8
`

	if b.String() != expected {
		t.Errorf("Found unexpected start output: %s", b.String())
	}
}

func TestPrintJobPending(t *testing.T) {
	setupRegistryForStart(1)

	jobs := map[string]bool{"echo.service": true, "pong.service": true}
	b := bytes.NewBufferString("")
	waitForScheduledUnits(jobs, 1, b)

	expected := `Job pong.service scheduled to c31e44e1.../1.2.3.4
Job echo.service still queued for scheduling
`

	if b.String() != expected {
		t.Errorf("Found unexpected start output: %s", b.String())
	}
}

func TestPrintJobWithoutPublicIP(t *testing.T) {
	setupRegistryForStart(0)

	jobs := map[string]bool{"private.service": true}
	b := bytes.NewBufferString("")
	waitForScheduledUnits(jobs, 1, b)

	if b.String() != "Job private.service scheduled to 520983A8...\n" {
		t.Errorf("Found unexpected start output: %s", b.String())
	}
}
