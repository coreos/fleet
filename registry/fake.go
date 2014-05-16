package registry

import (

	"github.com/coreos/fleet/third_party/github.com/coreos/go-semver/semver"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/unit"
)

func NewFakeRegistry(machines []machine.MachineState, jobStates map[string]*unit.UnitState, jobs []job.Job, units []unit.Unit, v *semver.Version) *FakeRegistry {
	return &FakeRegistry{machines: machines, jobStates: jobStates, jobs: jobs, units: units, version: v}
}

type FakeRegistry struct {
	// Not all methods of required by the Registry interface are implemented
	// by the TestRegistry. Any calls to these unimplemented methods will
	// result in a panic.
	Registry

	machines  []machine.MachineState
	jobStates map[string]*unit.UnitState
	jobs      []job.Job
	units     []unit.Unit
	version   *semver.Version
}

func (f *FakeRegistry) GetActiveMachines() ([]machine.MachineState, error) {
	return f.machines, nil
}

func (f *FakeRegistry) GetAllJobs() ([]job.Job, error) {
	return f.jobs, nil
}

func (f *FakeRegistry) GetJob(name string) (*job.Job, error) {
	for _, j := range f.jobs {
		if j.Name == name {
			j.UnitState = f.jobStates[name]
			return &j, nil
		}
	}
	return nil, nil
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
