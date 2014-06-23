package client

import (
	"github.com/coreos/fleet/third_party/github.com/coreos/go-semver/semver"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/sign"
)

type API interface {
	CreateJob(*job.Job) error
	CreateSignatureSet(*sign.SignatureSet) error
	DestroyJob(string) error
	GetJob(string) (*job.Job, error)
	GetJobTarget(string) (string, error)
	GetSignatureSetOfJob(string) (*sign.SignatureSet, error)
	Jobs() ([]job.Job, error)
	LatestVersion() (*semver.Version, error)
	Machines() ([]machine.MachineState, error)
	SetJobTargetState(string, job.JobState) error
}
