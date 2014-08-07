package agent

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/unit"
)

func newTestJobFromUnitContents(t *testing.T, name, contents string) *job.Job {
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

func newNamedTestJobWithXFleetValues(t *testing.T, name, metadata string) *job.Job {
	contents := fmt.Sprintf(`
[X-Fleet]
%s
`, metadata)
	return newTestJobFromUnitContents(t, name, contents)
}

func newTestJobWithXFleetValues(t *testing.T, metadata string) *job.Job {
	return newNamedTestJobWithXFleetValues(t, "pong.service", metadata)
}

func TestAgentLoadUnloadJob(t *testing.T) {
	uManager := unit.NewFakeUnitManager()
	usGenerator := unit.NewUnitStateGenerator(uManager)
	fReg := registry.NewFakeRegistry()
	mach := &machine.FakeMachine{machine.MachineState{ID: "XXX"}}
	a, err := New(uManager, usGenerator, fReg, mach, DefaultTTL)
	if err != nil {
		t.Fatalf("Failed creating Agent: %v", err)
	}

	j := newTestJobFromUnitContents(t, "foo.service", "")
	err = a.loadJob(j)
	if err != nil {
		t.Fatalf("Failed calling Agent.loadJob: %v", err)
	}

	jobs, err := a.jobs()
	if err != nil {
		t.Fatalf("Failed calling Agent.jobs: %v", err)
	}

	jsLoaded := job.JobStateLoaded
	expectJobs := map[string]*job.Job{
		"foo.service": &job.Job{
			Name: "foo.service",
			UnitState: &unit.UnitState{
				LoadState:   "loaded",
				ActiveState: "active",
				SubState:    "running",
				MachineID:   "",
				UnitName:    "foo.service",
			},
			State: &jsLoaded,

			Unit:            unit.Unit{},
			TargetState:     job.JobState(""),
			TargetMachineID: "",
		},
	}

	if !reflect.DeepEqual(expectJobs, jobs) {
		t.Fatalf("Received unexpected collection of Jobs: %#v\nExpected: %#v", jobs, expectJobs)
	}

	a.unloadJob("foo.service")

	// This sucks, but we have to do it if Agent.unloadJob is going to spin
	// off the real work that matters in a goroutine
	time.Sleep(200)

	jobs, err = a.jobs()
	if err != nil {
		t.Fatalf("Failed calling Agent.jobs: %v", err)
	}

	expectJobs = map[string]*job.Job{}
	if !reflect.DeepEqual(expectJobs, jobs) {
		t.Fatalf("Received unexpected collection of Jobs: %#v\nExpected: %#v", jobs, expectJobs)
	}
}

func TestAgentLoadStartStopJob(t *testing.T) {
	uManager := unit.NewFakeUnitManager()
	usGenerator := unit.NewUnitStateGenerator(uManager)
	fReg := registry.NewFakeRegistry()
	mach := &machine.FakeMachine{machine.MachineState{ID: "XXX"}}
	a, err := New(uManager, usGenerator, fReg, mach, DefaultTTL)
	if err != nil {
		t.Fatalf("Failed creating Agent: %v", err)
	}

	u, err := unit.NewUnit("")
	if err != nil {
		t.Fatalf("Failed creating Unit: %v", err)
	}

	j := job.NewJob("foo.service", *u)

	err = a.loadJob(j)
	if err != nil {
		t.Fatalf("Failed calling Agent.loadJob: %v", err)
	}

	a.startJob("foo.service")

	jobs, err := a.jobs()
	if err != nil {
		t.Fatalf("Failed calling Agent.jobs: %v", err)
	}

	jsLaunched := job.JobStateLaunched
	expectJobs := map[string]*job.Job{
		"foo.service": &job.Job{
			Name: "foo.service",
			UnitState: &unit.UnitState{
				LoadState:   "loaded",
				ActiveState: "active",
				SubState:    "running",
				MachineID:   "",
				UnitName:    "foo.service",
			},
			State: &jsLaunched,

			Unit:            unit.Unit{},
			TargetState:     job.JobState(""),
			TargetMachineID: "",
		},
	}

	if !reflect.DeepEqual(expectJobs, jobs) {
		t.Fatalf("Received unexpected collection of Jobs: %#v\nExpected: %#v", jobs, expectJobs)
	}

	a.stopJob("foo.service")

	jobs, err = a.jobs()
	if err != nil {
		t.Fatalf("Failed calling Agent.jobs: %v", err)
	}

	jsLoaded := job.JobStateLoaded
	expectJobs = map[string]*job.Job{
		"foo.service": &job.Job{
			Name: "foo.service",
			UnitState: &unit.UnitState{
				LoadState:   "loaded",
				ActiveState: "active",
				SubState:    "running",
				MachineID:   "",
				UnitName:    "foo.service",
			},
			State: &jsLoaded,

			Unit:            unit.Unit{},
			TargetState:     job.JobState(""),
			TargetMachineID: "",
		},
	}

	if !reflect.DeepEqual(expectJobs, jobs) {
		t.Fatalf("Received unexpected collection of Jobs: %#v\nExpected: %#v", jobs, expectJobs)
	}
}
