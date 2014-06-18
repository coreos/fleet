package agent

import (
	"fmt"
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/unit"
)

func newTestJobWithMachineMetadata(metadata string) *job.Job {
	contents := fmt.Sprintf(`
[X-Fleet]
%s
`, metadata)
	u, _ := unit.NewUnit(contents)

	return job.NewJob("pong.service", *u)
}

func TestAbleToRunConditionMachineIDMatch(t *testing.T) {
	u, _ := unit.NewUnit(`[X-Fleet]
X-ConditionMachineID=XYZ
`)
	job := job.NewJob("example.service", *u)

	mach := &machine.FakeMachine{machine.MachineState{ID: "XYZ"}}
	agent := Agent{Machine: mach, state: NewState()}
	if !agent.ableToRun(job) {
		t.Fatalf("Agent should be able to run job")
	}
}

func TestAbleToRunConditionMachineIDMismatch(t *testing.T) {
	u, _ := unit.NewUnit(`[X-Fleet]
X-ConditionMachineID=XYZ
`)
	job := job.NewJob("example.service", *u)

	mach := &machine.FakeMachine{machine.MachineState{ID: "123"}}
	agent := Agent{Machine: mach, state: NewState()}
	if agent.ableToRun(job) {
		t.Fatalf("Agent should not be able to run job")
	}
}

// Assert that an existing conflict is triggered against the potential job name
func TestHasConflictExistingMatch(t *testing.T) {
	state := NewState()

	u, _ := unit.NewUnit(`[X-Fleet]
X-Conflicts=other.service
`)
	j := job.NewJob("example.service", *u)
	state.TrackJob(j)
	state.SetTargetState(j.Name, job.JobStateLoaded)

	agent := Agent{state: state}

	matched, name := agent.HasConflict("other.service", []string{})
	if !matched {
		t.Errorf("Expected conflict with 'example.service', no conflict reported")
	} else if name != "example.service" {
		t.Errorf("Expected conflict with 'example.service', but conflict found with %s", name)
	}
}

// Assert that a potential conflict is triggered against the existing job name
func TestHasConflictPotentialMatch(t *testing.T) {
	state := NewState()

	u, _ := unit.NewUnit(`[X-Fleet]`)
	j := job.NewJob("example.service", *u)
	state.TrackJob(j)
	state.SetTargetState(j.Name, job.JobStateLoaded)

	agent := Agent{state: state}

	matched, name := agent.HasConflict("other.service", []string{"example.service"})
	if !matched {
		t.Errorf("Expected conflict with 'example.service', no conflict reported")
	} else if name != "example.service" {
		t.Errorf("Expected conflict with 'example.service', but conflict found with %s", name)
	}
}

// Assert that existing jobs and potential jobs that do not conflict do not
// trigger a match
func TestHasConflictNoMatch(t *testing.T) {
	state := NewState()

	u, _ := unit.NewUnit(`[X-Fleet]`)
	j := job.NewJob("example.service", *u)
	state.TrackJob(j)
	state.SetTargetState(j.Name, job.JobStateLoaded)

	agent := Agent{state: state}

	matched, name := agent.HasConflict("other.service", []string{})
	if matched {
		t.Errorf("Expected no match, but got conflict with %s", name)
	}
}

// Assert that our glob-parser can handle relatively-complex matching
func TestHasConflictComplexGlob(t *testing.T) {
	state := NewState()

	u, _ := unit.NewUnit(`[X-Fleet]
X-Conflicts=*.[1-9].service
`)
	j := job.NewJob("example.service", *u)
	state.TrackJob(j)
	state.SetTargetState(j.Name, job.JobStateLoaded)

	agent := Agent{state: state}

	matched, name := agent.HasConflict("other.2.service", []string{})
	if !matched {
		t.Errorf("Expected conflict with 'example.service', but no conflict reported")
	} else if name != "example.service" {
		t.Errorf("Expected conflict with 'example.service', but conflict found with %s", name)
	}
}

func TestHasConflictIgnoresUnscheduledJobs(t *testing.T) {
	state := NewState()

	u, _ := unit.NewUnit(`[X-Fleet]
X-Conflicts=other.service
`)
	j := job.NewJob("example.service", *u)
	state.TrackJob(j)
	state.SetTargetState(j.Name, job.JobStateInactive)

	agent := Agent{state: state}

	matched, name := agent.HasConflict("other.service", []string{})
	if matched {
		t.Errorf("Expected no conflict, but got conflict with %s", name)
	}
}

func TestHasConflictIgnoresBids(t *testing.T) {
	state := NewState()

	u, _ := unit.NewUnit(`[X-Fleet]
X-Conflicts=other.service
`)
	j := job.NewJob("example.service", *u)
	state.TrackJob(j)
	state.TrackBid(j.Name)

	agent := Agent{state: state}

	matched, name := agent.HasConflict("other.service", []string{})
	if matched {
		t.Errorf("Expected no conflict, but got conflict with %s", name)
	}
}

func TestAbleToRunWithConditionMachineMetadata(t *testing.T) {
	metadataAbleToRunExamples := []struct {
		C string
		A bool
	}{
		// valid metadata
		{`X-ConditionMachineMetadata=region=us-west-1`, true},
		{`X-ConditionMachineMetadata= "region=us-east-1" "region=us-west-1"`, true},
		{`X-ConditionMachineMetadata=region=us-east-1
X-ConditionMachineMetadata=region=us-west-1`, true},
		{`X-ConditionMachineMetadata=region=us-east-1`, false},

		// ignored/invalid metadata
		{`X-ConditionMachineMetadata=us-west-1`, true},
		{`X-ConditionMachineMetadata==us-west-1`, true},
		{`X-ConditionMachineMetadata=region=`, true},
	}

	metadata := map[string]string{
		"region": "us-west-1",
	}
	ms := machine.MachineState{Metadata: metadata}
	agent := &Agent{Machine: &machine.FakeMachine{ms}, state: NewState()}

	for i, e := range metadataAbleToRunExamples {
		job := newTestJobWithMachineMetadata(e.C)
		g := agent.ableToRun(job)
		if g != e.A {
			t.Errorf("Unexpected output %d, content: %q\n\tgot %t, want %t\n", i, e.C, g, e.A)
		}
	}
}
