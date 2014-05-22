package registry

import (

	"github.com/coreos/fleet/third_party/github.com/coreos/go-semver/semver"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/unit"
)

func NewTestRegistry(machines []machine.MachineState, jobStates map[string]*unit.UnitState, jobs []job.Job, units []unit.Unit, v *semver.Version) *TestRegistry {
	return &TestRegistry{machines: machines, jobStates: jobStates, jobs: jobs, units: units, version: v}
}

type TestRegistry struct {
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

func (t *TestRegistry) GetLatestVersion() (*semver.Version, error) {
	return t.version, nil
}
