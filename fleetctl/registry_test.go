package main

import (
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/sign"
	"github.com/coreos/fleet/unit"
)

type TestRegistry struct {
	machines  []machine.MachineState
	jobStates map[string]*unit.UnitState
	jobs      []job.Job
	units     []unit.Unit
}

func (t TestRegistry) GetActiveMachines() []machine.MachineState {
	return t.machines
}

func (t TestRegistry) GetAllJobs() []job.Job {
	return t.jobs
}

func (t TestRegistry) GetJob(name string) *job.Job {
	for _, j := range t.jobs {
		if j.Name == name {
			j.UnitState = t.jobStates[name]
			return &j
		}
	}
	return nil
}

func (m TestRegistry) SetJobTargetState(name string, target job.JobState) error {
	return nil
}

func (t TestRegistry) CreateJob(j *job.Job) error {
	return nil
}

func (t TestRegistry) DestroyJob(name string) {
}

func (t TestRegistry) CreateSignatureSet(s *sign.SignatureSet) error {
	return nil
}

func (t TestRegistry) GetSignatureSetOfPayload(name string) *sign.SignatureSet {
	return nil
}

func (t TestRegistry) GetSignatureSetOfJob(name string) *sign.SignatureSet {
	return nil
}

func (t TestRegistry) GetJobTarget(name string) string {
	js := t.jobStates[name]
	if js != nil {
		return js.MachineState.ID
	}
	return ""
}

func (t TestRegistry) GetMachineState(machID string) *machine.MachineState {
	for _, ms := range t.machines {
		if ms.ID == machID {
			return &ms
		}
	}
	return nil
}

func (t TestRegistry) GetDebugInfo() (string, error) {
	return "", nil
}
