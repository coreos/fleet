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
	CreateUnit(*job.Unit) error
	CreateSignatureSet(ss *sign.SignatureSet) error
	DestroyUnit(string) error
	DestroySignatureSet(tag string)
	JobHeartbeat(jobName, agentMachID string, ttl time.Duration) error
	JobSignatureSet(name string) (*sign.SignatureSet, error)
	LatestVersion() (*semver.Version, error)
	LeaseRole(role, machID string, period time.Duration) (Lease, error)
	Machines() ([]machine.MachineState, error)
	RemoveMachineState(machID string) error
	RemoveUnitState(jobName string) error
	SaveUnitState(jobName string, unitState *unit.UnitState)
	ScheduleUnit(name, machID string) error
	SetJobTargetState(jobName string, state job.JobState) error
	SetMachineState(ms machine.MachineState, ttl time.Duration) (uint64, error)

	UnitRegistry
}

type UnitRegistry interface {
	Schedule() ([]job.ScheduledUnit, error)
	ScheduledUnit(name string) (*job.ScheduledUnit, error)
	Unit(name string) (*job.Unit, error)
	Units() ([]job.Unit, error)
	UnitStates() ([]*unit.UnitState, error)
}

type Lease interface {
	Renew(time.Duration) error
	Release() error
}
