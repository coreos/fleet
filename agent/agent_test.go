package agent

import (
	"fmt"
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/unit"
)

func newTestMachine(region string) *machine.Machine {
	metadata := map[string]string{
		"region": region,
	}
	return machine.New("", "", metadata)
}

func newTestJobWithMachineMetadata(metadata string) *job.Job {
	contents := fmt.Sprintf(`
[X-Fleet]
%s
`, metadata)

	jp1 := job.NewJobPayload("us-west.service", *unit.NewSystemdUnitFile(contents))

	return job.NewJob("pong.service", *jp1)
}

func TestAbleToRunConditionMachineBootIDMatch(t *testing.T) {
	uf := unit.NewSystemdUnitFile(`[X-Fleet]
X-ConditionMachineBootID=XYZ
`)
	payload := job.NewJobPayload("example.service", *uf)
	job := job.NewJob("example.service", *payload)

	mach := machine.New("XYZ", "", make(map[string]string, 0))
	agent := Agent{machine: mach, state: NewState()}
	if !agent.AbleToRun(job) {
		t.Fatalf("Agent should be able to run job")
	}
}

func TestAbleToRunConditionMachineBootIDMismatch(t *testing.T) {
	uf := unit.NewSystemdUnitFile(`[X-Fleet]
X-ConditionMachineBootID=XYZ
`)
	payload := job.NewJobPayload("example.service", *uf)
	job := job.NewJob("example.service", *payload)

	mach := machine.New("123", "", make(map[string]string, 0))
	agent := Agent{machine: mach, state: NewState()}
	if agent.AbleToRun(job) {
		t.Fatalf("Agent should not be able to run job")
	}
}

// Assert that an existing conflict is triggered against the potential job name
func TestHasConflictExistingMatch(t *testing.T) {
	state := NewState()

	u := unit.NewSystemdUnitFile(`[X-Fleet]
X-Conflicts=other.service
`)
	p := job.NewJobPayload("example.service", *u)
	j := job.NewJob("example.service", *p)
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

	u := unit.NewSystemdUnitFile(`[X-Fleet]`)
	p := job.NewJobPayload("example.service", *u)
	j := job.NewJob("example.service", *p)
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

	u := unit.NewSystemdUnitFile(`[X-Fleet]`)
	p := job.NewJobPayload("example.service", *u)
	j := job.NewJob("example.service", *p)
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

	u := unit.NewSystemdUnitFile(`[X-Fleet]
X-Conflicts=*.[1-9].service
`)
	p := job.NewJobPayload("example.service", *u)
	j := job.NewJob("example.service", *p)
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

	agent := &Agent{machine: newTestMachine("us-west-1"), state: NewState()}

	for i, e := range metadataAbleToRunExamples {
		job := newTestJobWithMachineMetadata(e.C)
		g := agent.AbleToRun(job)
		if g != e.A {
			t.Errorf("Unexpected output %d, content: %q\n\tgot %q, want %q\n", i, e.C, g, e.A)
		}
	}
}
