package registry

import (
	"time"

	"github.com/coreos/fleet/Godeps/_workspace/src/github.com/coreos/go-semver/semver"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/sign"
	"github.com/coreos/fleet/unit"
)

type Registry interface {
	ClearJobHeartbeat(jobName string)
	ClearJobTarget(jobName, machID string) error
	CreateJob(j *job.Job) (err error)
	CreateSignatureSet(ss *sign.SignatureSet) error
	DestroyJob(jobName string) error
	DestroySignatureSet(tag string)
	Job(jobName string) (j *job.Job, err error)
	JobHeartbeat(jobName, agentMachID string, ttl time.Duration) error
	Jobs() ([]job.Job, error)
	JobSignatureSet(name string) (*sign.SignatureSet, error)
	LatestVersion() (*semver.Version, error)
	LeaseRole(role, machID string, period time.Duration) (Lease, error)
	Machines() ([]machine.MachineState, error)
	RemoveMachineState(machID string) error
	RemoveUnitState(jobName string) error
	SaveUnitState(jobName string, unitState *unit.UnitState)
	ScheduleJob(jobName string, machID string) error
	SetJobTargetState(jobName string, state job.JobState) error
	SetMachineState(ms machine.MachineState, ttl time.Duration) (uint64, error)

	UnitRegistry
}

type UnitRegistry interface {
	JobUnits() ([]job.JobUnit, error)
	Schedule() ([]job.ScheduledUnit, error)
	UnitStates() ([]*unit.UnitState, error)
}

type Lease interface {
	Renew(time.Duration) error
	Release() error
}
