package registry

import (

	"github.com/coreos/fleet/third_party/github.com/coreos/go-semver/semver"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/unit"
)

func NewFakeRegistry() *FakeRegistry {
	return &FakeRegistry{
		machines:  []machine.MachineState{},
		jobStates: map[string]*unit.UnitState{},
		jobs:      map[string]job.Job{},
		units:     []unit.Unit{},
		version:   nil,
	}
}

type FakeRegistry struct {
	// Not all methods of required by the Registry interface are implemented
	// by the TestRegistry. Any calls to these unimplemented methods will
	// result in a panic.
	Registry

	machines  []machine.MachineState
	jobStates map[string]*unit.UnitState
	jobs      map[string]job.Job
	units     []unit.Unit
	version   *semver.Version
}

func (f *FakeRegistry) SetMachines(machines []machine.MachineState) {
	f.machines = machines
}

func (f *FakeRegistry) SetJobs(jobs []job.Job) {
	f.jobs = make(map[string]job.Job, len(jobs))
	for _, j := range jobs {
		f.jobs[j.Name] = j
	}
}

func (f *FakeRegistry) SetUnitStates(jobStates map[string]*unit.UnitState) {
	f.jobStates = jobStates
}

func (f *FakeRegistry) SetUnits(units []unit.Unit) {
	f.units = units
}

func (f *FakeRegistry) SetLatestVersion(v semver.Version) {
	f.version = &v
}

func (f *FakeRegistry) GetActiveMachines() ([]machine.MachineState, error) {
	return f.machines, nil
}

func (f *FakeRegistry) GetAllJobs() ([]job.Job, error) {
	jobs := make([]job.Job, 0, len(f.jobs))
	for _, j := range f.jobs {
		jobs = append(jobs, j)
	}
	return jobs, nil
}

func (f *FakeRegistry) GetJob(name string) (*job.Job, error) {
	j, ok := f.jobs[name]
	if !ok {
		return nil, nil
	}

	j.UnitState = f.jobStates[name]
	return &j, nil
}

func (f *FakeRegistry) GetJobTarget(name string) (string, error) {
	js := f.jobStates[name]
	if js != nil {
		return js.MachineState.ID, nil
	}
	return "", nil
}

func (f *FakeRegistry) GetMachineState(machID string) (*machine.MachineState, error) {
	for _, ms := range f.machines {
		if ms.ID == machID {
			return &ms, nil
		}
	}
	return nil, nil
}

func (f *FakeRegistry) GetLatestVersion() (*semver.Version, error) {
	return f.version, nil
}
