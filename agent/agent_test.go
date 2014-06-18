package agent

import (
	"fmt"
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
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

func newTestJobWithXFleetValues(t *testing.T, metadata string) *job.Job {
	contents := fmt.Sprintf(`
[X-Fleet]
%s
`, metadata)
	return newTestJobFromUnitContents(t, "pong.service", contents)
}

func TestAbleToRunConditionMachineID(t *testing.T) {
	for id, want := range map[string]bool{
		"XYZ": true,
		"123": false,
	} {
		job := newTestJobWithXFleetValues(t, "X-ConditionMachineID=XYZ")
		mach := &machine.FakeMachine{machine.MachineState{ID: id}}
		agent := Agent{Machine: mach, state: NewState()}
		got := agent.ableToRun(job)
		if got != want {
			t.Errorf("Bad ableToRun for machineID %q: got %t, want %t", id, got, want)
		}

	}
}

func TestHasConflictMatches(t *testing.T) {
	for i, tt := range []struct {
		contents           string
		potentialConflicts []string
	}{
		{"[X-Fleet]\nX-Conflicts=other.service", []string{}},
		{"[X-Fleet]", []string{"example.service"}},
	} {
		state := NewState()
		j := newTestJobFromUnitContents(t, "example.service", tt.contents)
		state.TrackJob(j)
		state.SetTargetState(j.Name, job.JobStateLoaded)
		agent := Agent{state: state}
		matched, name := agent.HasConflict("other.service", []string{"example.service"})
		if !matched {
			t.Errorf("%d: Expected conflict with 'example.service', no conflict reported", i)
		} else if name != "example.service" {
			t.Errorf("%d: Expected conflict with 'example.service', but conflict found with %s", i, name)
		}
	}
}

// Assert that existing jobs and potential jobs that do not conflict do not
// trigger a match
func TestHasConflictNoMatch(t *testing.T) {
	state := NewState()
	j := newTestJobFromUnitContents(t, "example.service", "[X-Fleet]")
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

	j := newTestJobWithXFleetValues(t, "X-Conflicts=*.[1-9].service")
	state.TrackJob(j)
	state.SetTargetState(j.Name, job.JobStateLoaded)

	agent := Agent{state: state}

	for _, conflict := range []string{"other.2.service", "foo.1.service", ".9.service"} {
		matched, name := agent.HasConflict(conflict, []string{})
		if !matched {
			t.Errorf("Expected %q to have conflict with %q, but no conflict reported", conflict, j.Name)
		} else if name != j.Name {
			t.Errorf("Expected %q to have conflict with %q, but conflict found with %s", conflict, j.Name, name)
		}
	}
}

func TestHasConflictIgnoresUnscheduledJobs(t *testing.T) {
	state := NewState()
	j := newTestJobWithXFleetValues(t, "X-Conflicts=other.service")
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
	j := newTestJobWithXFleetValues(t, "X-Conflicts=other.service")
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
		job := newTestJobWithXFleetValues(t, e.C)
		g := agent.ableToRun(job)
		if g != e.A {
			t.Errorf("Unexpected output %d, content: %q\n\tgot %t, want %t\n", i, e.C, g, e.A)
		}
	}
}
