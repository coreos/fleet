package main

import (
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/sign"
)

type Registry interface {
	GetActiveMachines() []machine.MachineState
	GetAllJobs() []job.Job
	GetJob(name string) *job.Job
	CreateJob(j *job.Job) (err error)
	DestroyJob(name string)
	SetJobTargetState(name string, target job.JobState) error
	CreateSignatureSet(s *sign.SignatureSet) error
	GetSignatureSetOfPayload(name string) *sign.SignatureSet
	GetJobTarget(name string) string
	GetMachineState(bootID string) *machine.MachineState
	GetDebugInfo() (string, error)
}

type MainRegistry struct {
	registry *registry.Registry
}

func NewRegistry(r *registry.Registry) Registry {
	return MainRegistry{r}
}

func (m MainRegistry) GetActiveMachines() []machine.MachineState {
	return m.registry.GetActiveMachines()
}

func (m MainRegistry) GetAllJobs() []job.Job {
	return m.registry.GetAllJobs()
}

func (m MainRegistry) GetJob(name string) *job.Job {
	return m.registry.GetJob(name)
}

func (m MainRegistry) SetJobTargetState(name string, target job.JobState) error {
	return m.registry.SetJobTargetState(name, target)
}

func (m MainRegistry) DestroyJob(name string) {
	m.registry.DestroyJob(name)
}

func (m MainRegistry) CreateJob(j *job.Job) error {
	return m.registry.CreateJob(j)
}

func (m MainRegistry) CreateSignatureSet(s *sign.SignatureSet) error {
	return m.registry.CreateSignatureSet(s)
}

func (m MainRegistry) GetSignatureSetOfPayload(name string) *sign.SignatureSet {
	return m.registry.GetSignatureSetOfPayload(name)
}

func (m MainRegistry) GetJobTarget(name string) string {
	return m.registry.GetJobTarget(name)
}

func (m MainRegistry) GetMachineState(bootID string) *machine.MachineState {
	return m.registry.GetMachineState(bootID)
}

func (m MainRegistry) GetDebugInfo() (string, error) {
	return m.registry.GetDebugInfo()
}
