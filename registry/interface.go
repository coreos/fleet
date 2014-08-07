package registry

import (
	"time"

	"github.com/coreos/fleet/Godeps/_workspace/src/github.com/coreos/go-semver/semver"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/pkg"
	"github.com/coreos/fleet/unit"
)

type Registry interface {
	Bids(jobName string) (pkg.Set, error)
	ClearJobHeartbeat(jobName string)
	ClearJobTarget(jobName, machID string) error
	CreateJob(j *job.Job) (err error)
	CreateJobOffer(jo *job.JobOffer) error
	DestroyJob(jobName string) error
	Job(jobName string) (j *job.Job, err error)
	JobHeartbeat(jobName, agentMachID string, ttl time.Duration) error
	Jobs() ([]job.Job, error)
	LatestVersion() (*semver.Version, error)
	LeaseRole(role, machID string, period time.Duration) (Lease, error)
	Machines() ([]machine.MachineState, error)
	RemoveMachineState(machID string) error
	RemoveUnitState(jobName string) error
	ResolveJobOffer(jobName string) error
	SaveUnitState(jobName string, unitState *unit.UnitState)
	ScheduleJob(jobName string, machID string) error
	SetJobTargetState(jobName string, state job.JobState) error
	SetMachineState(ms machine.MachineState, ttl time.Duration) (uint64, error)
	SubmitJobBid(jName, machID string)
	UnresolvedJobOffers() ([]job.JobOffer, error)
}

type UnitStateRegistry interface {
	States() ([]*unit.UnitState, error)
}

type Lease interface {
	Renew(time.Duration) error
	Release() error
}
