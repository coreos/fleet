package main

import (
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/sign"
)

type TestRegistry struct {
	machines  []machine.MachineState
	jobStates map[string]*job.PayloadState
	jobs      []job.Job
	payloads  []job.JobPayload
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
			j.PayloadState = t.jobStates[name]
			return &j
		}
	}
	return nil
}

func (t TestRegistry) StartJob(name string) {
}

func (t TestRegistry) StopJob(name string) {
}

func (t TestRegistry) DestroyJob(name string) {
}

func (t TestRegistry) CreateJob(j *job.Job) error {
	return nil
}

func (t TestRegistry) CreateSignatureSet(s *sign.SignatureSet) error {
	return nil
}

func (t TestRegistry) GetSignatureSetOfPayload(name string) *sign.SignatureSet {
	return nil
}

func (t TestRegistry) GetJobTarget(name string) string {
	js := t.jobStates[name]
	if js != nil {
		return js.MachineState.BootID
	}
	return ""
}

func (t TestRegistry) GetMachineState(bootID string) *machine.MachineState {
	for _, ms := range t.machines {
		if ms.BootID == bootID {
			return &ms
		}
	}
	return nil
}

func (t TestRegistry) GetDebugInfo() (string, error) {
	return "", nil
}
