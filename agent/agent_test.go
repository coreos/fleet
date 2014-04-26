package agent

import (
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/unit"
)

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

// Assert that a existing jobs and potential jobs that do not conflict do not
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
		t.Errorf("Expected conflict with 'other.2.service', no conflict reported")
	} else if name != "example.service" {
		t.Errorf("Expected conflict with 'other.2.service', but conflict found with %s", name)
	}

}
