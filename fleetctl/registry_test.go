package main

import (
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/sign"
)

type TestRegistry struct {
	machines  []machine.MachineState
	jobStates map[string]*job.JobState
	jobs      []job.Job
	payloads  []job.JobPayload
}

func (t TestRegistry) GetActiveMachines() []machine.MachineState {
	return t.machines
}

func (t TestRegistry) GetJobState(name string) *job.JobState {
	return t.jobStates[name]
}

func (t TestRegistry) GetAllPayloads() []job.JobPayload {
	return t.payloads
}

func (t TestRegistry) GetAllJobs() []job.Job {
	return t.jobs
}

func (t TestRegistry) GetPayload(name string) *job.JobPayload {
	return nil
}

func (t TestRegistry) StopJob(name string) {
}

func (t TestRegistry) DestroyPayload(name string) {
}

func (t TestRegistry) CreatePayload(jp *job.JobPayload) error {
	return nil
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

func (t TestRegistry) DestroySignatureSetOfPayload(name string) {
}

func (t TestRegistry) GetJobTarget(name string) *machine.MachineState {
	js := t.jobStates[name]
	if js != nil {
		return js.MachineState
	}
	return nil
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
