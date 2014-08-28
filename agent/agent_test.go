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

func newTestUnitFromUnitContents(t *testing.T, name, contents string) *job.Unit {
	j := newTestJobFromUnitContents(t, name, contents)
	return &job.Unit{
		Name: j.Name,
		Unit: j.Unit,
	}
}

func newTestJobFromUnitContents(t *testing.T, name, contents string) *job.Job {
	u, err := unit.NewUnitFile(contents)
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

func TestAgentLoadUnloadUnit(t *testing.T) {
	uManager := unit.NewFakeUnitManager()
	usGenerator := unit.NewUnitStateGenerator(uManager)
	fReg := registry.NewFakeRegistry()
	mach := &machine.FakeMachine{machine.MachineState{ID: "XXX"}}
	a := New(uManager, usGenerator, fReg, mach, time.Second)

	u := newTestUnitFromUnitContents(t, "foo.service", "")
	err := a.loadUnit(u)
	if err != nil {
		t.Fatalf("Failed calling Agent.loadUnit: %v", err)
	}

	jobs, err := a.jobs()
	if err != nil {
		t.Fatalf("Failed calling Agent.jobs: %v", err)
	}

	jsLoaded := job.JobStateLoaded
	expectJobs := map[string]*job.Job{
		"foo.service": &job.Job{
			Name:  "foo.service",
			State: &jsLoaded,

			Unit:            unit.UnitFile{},
			TargetState:     job.JobState(""),
			TargetMachineID: "",
		},
	}

	if !reflect.DeepEqual(expectJobs, jobs) {
		t.Fatalf("Received unexpected collection of Jobs: %#v\nExpected: %#v", jobs, expectJobs)
	}

	a.unloadUnit("foo.service")

	jobs, err = a.jobs()
	if err != nil {
		t.Fatalf("Failed calling Agent.jobs: %v", err)
	}

	expectJobs = map[string]*job.Job{}
	if !reflect.DeepEqual(expectJobs, jobs) {
		t.Fatalf("Received unexpected collection of Jobs: %#v\nExpected: %#v", jobs, expectJobs)
	}
}

func TestAgentLoadStartStopUnit(t *testing.T) {
	uManager := unit.NewFakeUnitManager()
	usGenerator := unit.NewUnitStateGenerator(uManager)
	fReg := registry.NewFakeRegistry()
	mach := &machine.FakeMachine{machine.MachineState{ID: "XXX"}}
	a := New(uManager, usGenerator, fReg, mach, time.Second)

	u := newTestUnitFromUnitContents(t, "foo.service", "")

	err := a.loadUnit(u)
	if err != nil {
		t.Fatalf("Failed calling Agent.loadUnit: %v", err)
	}

	a.startUnit("foo.service")

	jobs, err := a.jobs()
	if err != nil {
		t.Fatalf("Failed calling Agent.jobs: %v", err)
	}

	jsLaunched := job.JobStateLaunched
	expectJobs := map[string]*job.Job{
		"foo.service": &job.Job{
			Name:  "foo.service",
			State: &jsLaunched,

			Unit:            unit.UnitFile{},
			TargetState:     job.JobState(""),
			TargetMachineID: "",
		},
	}

	if !reflect.DeepEqual(expectJobs, jobs) {
		t.Fatalf("Received unexpected collection of Jobs: %#v\nExpected: %#v", jobs, expectJobs)
	}

	a.stopUnit("foo.service")

	jobs, err = a.jobs()
	if err != nil {
		t.Fatalf("Failed calling Agent.jobs: %v", err)
	}

	jsLoaded := job.JobStateLoaded
	expectJobs = map[string]*job.Job{
		"foo.service": &job.Job{
			Name:  "foo.service",
			State: &jsLoaded,

			Unit:            unit.UnitFile{},
			TargetState:     job.JobState(""),
			TargetMachineID: "",
		},
	}

	if !reflect.DeepEqual(expectJobs, jobs) {
		t.Fatalf("Received unexpected collection of Jobs: %#v\nExpected: %#v", jobs, expectJobs)
	}
}
