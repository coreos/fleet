package registry

import (
	"time"

	"github.com/coreos/fleet/Godeps/_workspace/src/github.com/coreos/go-semver/semver"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/unit"
)

type Registry interface {
	ClearUnitHeartbeat(name string)
	CreateUnit(*job.Unit) error
	DestroyUnit(string) error
	UnitHeartbeat(name, machID string, ttl time.Duration) error
	Machines() ([]machine.MachineState, error)
	RemoveMachineState(machID string) error
	RemoveUnitState(jobName string) error
	SaveUnitState(jobName string, unitState *unit.UnitState, ttl time.Duration)
	ScheduleUnit(name, machID string) error
	SetUnitTargetState(name string, state job.JobState) error
	SetMachineState(ms machine.MachineState, ttl time.Duration) (uint64, error)
	UnscheduleUnit(name, machID string) error

	UnitRegistry
}

type UnitRegistry interface {
	Schedule() ([]job.ScheduledUnit, error)
	ScheduledUnit(name string) (*job.ScheduledUnit, error)
	Unit(name string) (*job.Unit, error)
	Units() ([]job.Unit, error)
	UnitStates() ([]*unit.UnitState, error)
}

type ClusterRegistry interface {
	LatestDaemonVersion() (*semver.Version, error)

	// EngineVersion returns the current version of the cluster. If the
	// cluster does not yet have a version, zero will be returned. If
	// any error occurs, an error object will be returned. In this case,
	// the returned version number should be ignored.
	EngineVersion() (int, error)

	// UpdateEngineVersion attempts to compare-and-swap the cluster version
	// from one value to another. Any failures in this process will be
	// indicated by the returned error object. A nil value will be returned
	// on success.
	UpdateEngineVersion(from, to int) error
}

type LeaseRegistry interface {
	// AcquireLease acquires a named lease only if the lease is not
	// currently held. If a Lease cannot be acquired, a nil Lease
	// object is returned. An error is returned only if there is a
	// failure communicating with the Registry.
	AcquireLease(name, machID string, period time.Duration) (Lease, error)
}

// Lease proxies to an auto-expiring lease stored in a LeaseRegistry.
// The creator of a Lease must repeatedly call Renew to keep their lease
// from expiring.
type Lease interface {
	// Renew attempts to extend the Lease TTL to the provided duration.
	// The operation will succeed only if the Lease has not changed in
	// the LeaseRegistry since it was last renewed or first acquired.
	// An error is returned if the Lease has already expired, or if the
	// operation fails for any other reason.
	Renew(time.Duration) error

	// Release relinquishes the ownership of a Lease back to the Registry.
	// After calling Release, the Lease object should be discarded. An
	// error is returned if the Lease has already expired, or if the
	// operation fails for any other reason.
	Release() error
}
