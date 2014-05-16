package registry

import (
	"time"

	"github.com/coreos/fleet/third_party/github.com/coreos/go-semver/semver"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/sign"
	"github.com/coreos/fleet/unit"
)

func NewFakeRegistry(machines []machine.MachineState, jobStates map[string]*unit.UnitState, jobs []job.Job, units []unit.Unit) *FakeRegistry {
	return &FakeRegistry{machines, jobStates, jobs, units}
}

type FakeRegistry struct {
	machines  []machine.MachineState
	jobStates map[string]*unit.UnitState
	jobs      []job.Job
	units     []unit.Unit
	version   *semver.Version
}

func (t *FakeRegistry) GetActiveMachines() ([]machine.MachineState, error) {
	return t.machines, nil
}

func (t *FakeRegistry) GetAllJobs() ([]job.Job, error) {
	return t.jobs, nil
}

func (t *FakeRegistry) GetJob(name string) (*job.Job, error) {
	for _, j := range t.jobs {
		if j.Name == name {
			j.UnitState = t.jobStates[name]
			return &j, nil
		}
	}
	return nil, nil
}

func (t *FakeRegistry) SetJobTargetState(name string, target job.JobState) error {
	panic("Not implemented")
}

func (t *FakeRegistry) CheckJobPulse(jobName string) (string, bool) {
	panic("Not implemented")
}

func (t *FakeRegistry) CreateJob(j *job.Job) error {
	panic("Not implemented")
}

func (t *FakeRegistry) DestroyJob(name string) error {
	panic("Not implemented")
}

func (t *FakeRegistry) CreateSignatureSet(s *sign.SignatureSet) error {
	panic("Not implemented")
}

func (t *FakeRegistry) GetJobTarget(name string) (string, error) {
	js := t.jobStates[name]
	if js != nil {
		return js.MachineState.ID, nil
	}
	return "", nil
}

func (t *FakeRegistry) GetMachineState(machID string) (*machine.MachineState, error) {
	for _, ms := range t.machines {
		if ms.ID == machID {
			return &ms, nil
		}
	}
	return nil, nil
}

func (t *FakeRegistry) GetDebugInfo() (string, error) {
	return "", nil
}

func (t *FakeRegistry) ClearJobHeartbeat(jobName string) {
	panic("Not implemented")
}

func (t *FakeRegistry) ClearJobTarget(jobName, machID string) error {
	panic("Not implemented")
}

func (t *FakeRegistry) CreateJobOffer(jo *job.JobOffer) error {
	panic("Not implemented")
}

func (t *FakeRegistry) DestroySignatureSet(tag string) {
	panic("Not implemented")
}

func (t *FakeRegistry) GetJobTargetState(jobName string) (*job.JobState, error) {
	panic("Not implemented")
}

func (t *FakeRegistry) GetSignatureSet(tag string) *sign.SignatureSet {
	panic("Not implemented")
}

func (t *FakeRegistry) GetSignatureSetOfJob(name string) (*sign.SignatureSet, error) {
	panic("Not implemented")
}

func (t *FakeRegistry) JobHeartbeat(jobName, agentMachID string, ttl time.Duration) error {
	panic("Not implemented")
}

func (t *FakeRegistry) LockJob(jobName, context string) *TimedResourceMutex {
	panic("Not implemented")
}

func (t *FakeRegistry) LockJobOffer(jobName, context string) *TimedResourceMutex {
	panic("Not implemented")
}

func (t *FakeRegistry) LockMachine(machID, context string) *TimedResourceMutex {
	panic("Not implemented")
}

func (t *FakeRegistry) RemoveMachineState(machID string) error {
	panic("Not implemented")
}

func (t *FakeRegistry) RemoveUnitState(jobName string) error {
	panic("Not implemented")
}

func (t *FakeRegistry) ResolveJobOffer(jobName string) error {
	panic("Not implemented")
}

func (t *FakeRegistry) SaveUnitState(jobName string, unitState *unit.UnitState) {
	panic("Not implemented")
}

func (t *FakeRegistry) ScheduleJob(jobName string, machID string) error {
	panic("Not implemented")
}

func (t *FakeRegistry) SetMachineState(ms machine.MachineState, ttl time.Duration) (uint64, error) {
	panic("Not implemented")
}

func (t *FakeRegistry) SubmitJobBid(jb *job.JobBid) {
	panic("Not implemented")
}

func (t *FakeRegistry) UnresolvedJobOffers() []job.JobOffer {
	panic("Not implemented")
}

func (t *FakeRegistry) Bids(jo *job.JobOffer) ([]job.JobBid, error) {
	panic("Not implemented")
}

func (t *FakeRegistry) GetLatestVersion() (*semver.Version, error) {
	return t.version, nil
}
