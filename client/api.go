package client

import (
	"github.com/coreos/fleet/Godeps/_workspace/src/github.com/coreos/go-semver/semver"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/sign"
	"github.com/coreos/fleet/unit"
)

type API interface {
	CreateUnit(*job.Unit) error
	DestroyUnit(string) error
	CreateSignatureSet(*sign.SignatureSet) error
	JobSignatureSet(string) (*sign.SignatureSet, error)
	LatestVersion() (*semver.Version, error)
	Machines() ([]machine.MachineState, error)
	SetUnitTargetState(name string, target job.JobState) error

	Schedule() ([]job.ScheduledUnit, error)
	ScheduledUnit(name string) (*job.ScheduledUnit, error)
	Unit(string) (*job.Unit, error)
	Units() ([]job.Unit, error)
	UnitStates() ([]*unit.UnitState, error)
}
