package registry

import (
	"time"

	"github.com/coreos/fleet/third_party/github.com/coreos/go-semver/semver"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/sign"
	"github.com/coreos/fleet/unit"
)

func NewTestRegistry(machines []machine.MachineState, jobStates map[string]*unit.UnitState, jobs []job.Job, units []unit.Unit) *TestRegistry {
	return &TestRegistry{machines, jobStates, jobs, units}
}

type TestRegistry struct {
	machines  []machine.MachineState
	jobStates map[string]*unit.UnitState
	jobs      []job.Job
	units     []unit.Unit
	version   *semver.Version
}

func (t *TestRegistry) GetActiveMachines() ([]machine.MachineState, error) {
	return t.machines, nil
}

func (t *TestRegistry) GetAllJobs() ([]job.Job, error) {
	return t.jobs, nil
}

func (t *TestRegistry) GetJob(name string) (*job.Job, error) {
	for _, j := range t.jobs {
		if j.Name == name {
			j.UnitState = t.jobStates[name]
			return &j, nil
		}
	}
	return nil, nil
}

func (t *TestRegistry) SetJobTargetState(name string, target job.JobState) error {
	panic("Not implemented")
}

func (t *TestRegistry) CheckJobPulse(jobName string) (string, bool) {
	panic("Not implemented")
}

func (t *TestRegistry) CreateJob(j *job.Job) error {
	panic("Not implemented")
}

func (t *TestRegistry) DestroyJob(name string) error {
	panic("Not implemented")
}

func (t *TestRegistry) CreateSignatureSet(s *sign.SignatureSet) error {
	panic("Not implemented")
}

func (t *TestRegistry) GetJobTarget(name string) (string, error) {
	js := t.jobStates[name]
	if js != nil {
		return js.MachineState.ID, nil
	}
	return "", nil
}

func (t *TestRegistry) GetMachineState(machID string) (*machine.MachineState, error) {
	for _, ms := range t.machines {
		if ms.ID == machID {
			return &ms, nil
		}
	}
	return nil, nil
}

func (t *TestRegistry) GetDebugInfo() (string, error) {
	return "", nil
}

func (t *TestRegistry) ClearJobHeartbeat(jobName string) {
	panic("Not implemented")
}

func (t *TestRegistry) ClearJobTarget(jobName, machID string) error {
	panic("Not implemented")
}

func (t *TestRegistry) CreateJobOffer(jo *job.JobOffer) error {
	panic("Not implemented")
}

func (t *TestRegistry) DestroySignatureSet(tag string) {
	panic("Not implemented")
}

func (t *TestRegistry) GetJobTargetState(jobName string) (*job.JobState, error) {
	panic("Not implemented")
}

func (t *TestRegistry) GetSignatureSet(tag string) *sign.SignatureSet {
	panic("Not implemented")
}

func (t *TestRegistry) GetSignatureSetOfJob(name string) (*sign.SignatureSet, error) {
	panic("Not implemented")
}

func (t *TestRegistry) JobHeartbeat(jobName, agentMachID string, ttl time.Duration) error {
	panic("Not implemented")
}

func (t *TestRegistry) LockJob(jobName, context string) *TimedResourceMutex {
	panic("Not implemented")
}

func (t *TestRegistry) LockJobOffer(jobName, context string) *TimedResourceMutex {
	panic("Not implemented")
}

func (t *TestRegistry) LockMachine(machID, context string) *TimedResourceMutex {
	panic("Not implemented")
}

func (t *TestRegistry) RemoveMachineState(machID string) error {
	panic("Not implemented")
}

func (t *TestRegistry) RemoveUnitState(jobName string) error {
	panic("Not implemented")
}

func (t *TestRegistry) ResolveJobOffer(jobName string) error {
	panic("Not implemented")
}

func (t *TestRegistry) SaveUnitState(jobName string, unitState *unit.UnitState) {
	panic("Not implemented")
}

func (t *TestRegistry) ScheduleJob(jobName string, machID string) error {
	panic("Not implemented")
}

func (t *TestRegistry) SetMachineState(ms machine.MachineState, ttl time.Duration) (uint64, error) {
	panic("Not implemented")
}

func (t *TestRegistry) SubmitJobBid(jb *job.JobBid) {
	panic("Not implemented")
}

func (t *TestRegistry) UnresolvedJobOffers() []job.JobOffer {
	panic("Not implemented")
}

func (t *TestRegistry) Bids(jo *job.JobOffer) ([]job.JobBid, error) {
	panic("Not implemented")
}

func (t TestRegistry) GetLatestVersion() (*semver.Version, error) {
	return t.version, nil
}
