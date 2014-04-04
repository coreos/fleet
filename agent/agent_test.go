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
	job := job.NewJob("example.service", make(map[string][]string, 0), payload, nil)

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
	job := job.NewJob("example.service", make(map[string][]string, 0), payload, nil)

	mach := machine.New("123", "", make(map[string]string, 0))
	agent := Agent{machine: mach, state: NewState()}
	if agent.AbleToRun(job) {
		t.Fatalf("Agent should not be able to run job")
	}
}
