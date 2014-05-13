package main

import (
	"time"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/sign"
	"github.com/coreos/fleet/unit"
)

type TestRegistry struct {
	machines  []machine.MachineState
	jobStates map[string]*unit.UnitState
	jobs      []job.Job
	units     []unit.Unit
}

func (t TestRegistry) GetActiveMachines() ([]machine.MachineState, error) {
	return t.machines, nil
}

func (t TestRegistry) GetAllJobs() ([]job.Job, error) {
	return t.jobs, nil
}

func (t TestRegistry) GetJob(name string) (*job.Job, error) {
	for _, j := range t.jobs {
		if j.Name == name {
			j.UnitState = t.jobStates[name]
			return &j, nil
		}
	}
	return nil, nil
}

func (t TestRegistry) SetJobTargetState(name string, target job.JobState) error {
	return nil
}

func (t TestRegistry) CheckJobPulse(jobName string) (string, bool) {
	return "", false
}

func (t TestRegistry) CreateJob(j *job.Job) error {
	return nil
}

func (t TestRegistry) DestroyJob(name string) error {
	return nil
}

func (t TestRegistry) CreateSignatureSet(s *sign.SignatureSet) error {
	return nil
}

func (t TestRegistry) GetJobTarget(name string) (string, error) {
	js := t.jobStates[name]
	if js != nil {
		return js.MachineState.ID, nil
	}
	return "", nil
}

func (t TestRegistry) GetMachineState(machID string) (*machine.MachineState, error) {
	for _, ms := range t.machines {
		if ms.ID == machID {
			return &ms, nil
		}
	}
	return nil, nil
}

func (t TestRegistry) GetDebugInfo() (string, error) {
	return "", nil
}

func (t TestRegistry) ClearJobHeartbeat(jobName string) {
	return
}

func (t TestRegistry) ClearJobTarget(jobName, machID string) error {
	return nil
}

func (t TestRegistry) CreateJobOffer(jo *job.JobOffer) error {
	return nil
}

func (t TestRegistry) DestroySignatureSet(tag string) {
	return
}

func (t TestRegistry) GetJobTargetState(jobName string) (*job.JobState, error) {
	return nil, nil
}

func (t TestRegistry) GetSignatureSet(tag string) *sign.SignatureSet {
	return nil
}

func (t TestRegistry) GetSignatureSetOfJob(name string) (*sign.SignatureSet, error) {
	return nil, nil
}

func (t TestRegistry) JobHeartbeat(jobName, agentMachID string, ttl time.Duration) error {
	return nil
}

func (t TestRegistry) LockJob(jobName, context string) *registry.TimedResourceMutex {
	return nil
}

func (t TestRegistry) LockJobOffer(jobName, context string) *registry.TimedResourceMutex {
	return nil
}

func (t TestRegistry) LockMachine(machID, context string) *registry.TimedResourceMutex {
	return nil
}

func (t TestRegistry) RemoveMachineState(machID string) error {
	return nil
}

func (t TestRegistry) RemoveUnitState(jobName string) error {
	return nil
}

func (t TestRegistry) ResolveJobOffer(jobName string) error {
	return nil
}

func (t TestRegistry) SaveUnitState(jobName string, unitState *unit.UnitState) {
	return
}

func (t TestRegistry) ScheduleJob(jobName string, machID string) error {
	return nil
}

func (t TestRegistry) SetMachineState(ms machine.MachineState, ttl time.Duration) (uint64, error) {
	return 0, nil
}

func (t TestRegistry) SubmitJobBid(jb *job.JobBid) {
	return
}

func (t TestRegistry) UnresolvedJobOffers() []job.JobOffer {
	return nil
}

func (t TestRegistry) Bids(jo *job.JobOffer) ([]job.JobBid, error) {
	return nil, nil
}
