package main

import (
	"github.com/coreos/fleet/third_party/github.com/coreos/go-semver/semver"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/sign"
)

type FleetController interface {
	CreateJob(*job.Job) error
	CreateSignatureSet(*sign.SignatureSet) error
	DestroyJob(string) error
	GetActiveMachines() ([]machine.MachineState, error)
	GetAllJobs() ([]job.Job, error)
	GetJob(string) (*job.Job, error)
	GetJobTarget(string) (string, error)
	GetLatestVersion() (*semver.Version, error)
	GetMachineState(string) (*machine.MachineState, error)
	GetSignatureSetOfJob(string) (*sign.SignatureSet, error)
	SetJobTargetState(string, job.JobState) error
}
