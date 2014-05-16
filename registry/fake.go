package registry

import (
	"errors"
	"time"

	"github.com/coreos/fleet/third_party/github.com/coreos/go-semver/semver"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/sign"
	"github.com/coreos/fleet/unit"
)

func NewFakeRegistry() *FakeRegistry {
	return &FakeRegistry{
		machines:  []machine.MachineState{},
		jobStates: map[string]*unit.UnitState{},
		jobs:      map[string]job.Job{},
		units:     []unit.Unit{},
		bids:      map[string][]job.JobBid{},
	}
}

type FakeRegistry struct {
	machines  []machine.MachineState
	jobStates map[string]*unit.UnitState
	jobs      map[string]job.Job
	units     []unit.Unit
	version   *semver.Version
	bids      map[string][]job.JobBid
}

func (t *FakeRegistry) SetMachines(machines []machine.MachineState) {
	t.machines = machines
}

func (t *FakeRegistry) SetJobs(jobs []job.Job) {
	t.jobs = make(map[string]job.Job, len(jobs))
	for _, j := range jobs {
		t.jobs[j.Name] = j
	}
}

func (t *FakeRegistry) SetUnitStates(jobStates map[string]*unit.UnitState) {
	t.jobStates = jobStates
}

func (t *FakeRegistry) SetUnits(units []unit.Unit) {
	t.units = units
}

func (t *FakeRegistry) SetLatestVersion(v semver.Version) {
	t.version = &v
}

func (t *FakeRegistry) GetActiveMachines() ([]machine.MachineState, error) {
	return t.machines, nil
}

func (t *FakeRegistry) GetAllJobs() ([]job.Job, error) {
	jobs := make([]job.Job, 0, len(t.jobs))
	for _, j := range t.jobs {
		jobs = append(jobs, j)
	}
	return jobs, nil
}

func (t *FakeRegistry) GetJob(name string) (*job.Job, error) {
	j, ok := t.jobs[name]
	if !ok {
		return nil, nil
	}

	j.UnitState = t.jobStates[name]
	return &j, nil
}

func (t *FakeRegistry) SetJobTargetState(name string, target job.JobState) error {
	panic("Not implemented")
}

func (t *FakeRegistry) CheckJobPulse(jobName string) (string, bool) {
	panic("Not implemented")
}

func (t *FakeRegistry) CreateJob(j *job.Job) error {
	_, ok := t.jobs[j.Name]
	if ok {
		return errors.New("Job already exists")
	}

	t.jobs[j.Name] = *j
	return nil
}

func (t *FakeRegistry) DestroyJob(name string) error {
	delete(t.jobs, name)
	return nil
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

func (t *FakeRegistry) Bids(jo *job.JobOffer) ([]job.JobBid, error) {
	return t.bids[jo.Job.Name], nil
}

func (t *FakeRegistry) SubmitJobBid(jb *job.JobBid) {
	_, ok := t.bids[jb.JobName]
	if !ok {
		t.bids[jb.JobName] = []job.JobBid{}
	}
	t.bids[jb.JobName] = append(t.bids[jb.JobName], *jb)
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

func (t *FakeRegistry) UnresolvedJobOffers() []job.JobOffer {
	panic("Not implemented")
}

func (t *FakeRegistry) Bids(jo *job.JobOffer) ([]job.JobBid, error) {
	panic("Not implemented")
}

func (t *FakeRegistry) GetLatestVersion() (*semver.Version, error) {
	return t.version, nil
}
