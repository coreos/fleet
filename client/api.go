package client

import (
	"github.com/coreos/fleet/Godeps/_workspace/src/github.com/coreos/go-semver/semver"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
)

type API interface {
	CreateJob(*job.Job) error
	DestroyJob(string) error
	Job(string) (*job.Job, error)
	Jobs() ([]job.Job, error)
	LatestVersion() (*semver.Version, error)
	Machines() ([]machine.MachineState, error)
	SetJobTargetState(string, job.JobState) error
}
