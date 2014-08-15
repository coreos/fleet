package client

import (
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/schema"
	"github.com/coreos/fleet/sign"
)

type API interface {
	CreateSignatureSet(*sign.SignatureSet) error
	JobSignatureSet(string) (*sign.SignatureSet, error)

	Machines() ([]machine.MachineState, error)

	Unit(string) (*schema.Unit, error)
	Units() ([]*schema.Unit, error)
	UnitStates() ([]*schema.UnitState, error)

	SetUnitTargetState(name, target string) error
	CreateUnit(*schema.Unit) error
	DestroyUnit(string) error
}
