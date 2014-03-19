package main

import (
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/sign"
)

type Registry interface {
	GetActiveMachines() []machine.MachineState
	GetJobState(name string) *job.JobState
	GetAllPayloads() []job.JobPayload
	GetAllJobs() []job.Job
	GetPayload(name string) *job.JobPayload
	StopJob(name string)
	DestroyPayload(name string)
	CreatePayload(jp *job.JobPayload) error
	CreateJob(j *job.Job) (err error)
	CreateSignatureSet(s *sign.SignatureSet) error
	GetSignatureSetOfPayload(name string) *sign.SignatureSet
	DestroySignatureSetOfPayload(name string)
	GetJobTarget(name string) *machine.MachineState
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

func (m MainRegistry) GetJobState(name string) *job.JobState {
	return m.registry.GetJobState(name)
}

func (m MainRegistry) GetAllPayloads() []job.JobPayload {
	return m.registry.GetAllPayloads()
}

func (m MainRegistry) GetAllJobs() []job.Job {
	return m.registry.GetAllJobs()
}

func (m MainRegistry) GetPayload(name string) *job.JobPayload {
	return m.registry.GetPayload(name)
}

func (m MainRegistry) StopJob(name string) {
	m.registry.StopJob(name)
}

func (m MainRegistry) DestroyPayload(name string) {
	m.registry.DestroyPayload(name)
}

func (m MainRegistry) CreatePayload(jp *job.JobPayload) error {
	return m.registry.CreatePayload(jp)
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

func (m MainRegistry) DestroySignatureSetOfPayload(name string) {
	m.registry.DestroySignatureSetOfPayload(name)
}

func (m MainRegistry) GetJobTarget(name string) *machine.MachineState {
	return m.registry.GetJobTarget(name)
}

func (m MainRegistry) GetMachineState(bootID string) *machine.MachineState {
	return m.registry.GetMachineState(bootID)
}

func (m MainRegistry) GetDebugInfo() (string, error) {
	return m.registry.GetDebugInfo()
}
