package unit

import (
	"github.com/coreos/fleet/pkg"
)

type UnitManager interface {
	Load(string, Unit) error
	Unload(string)

	Start(string)
	Stop(string)

	Units() ([]string, error)
	GetUnitStates(pkg.Set) (map[string]*UnitState, error)
	GetUnitState(string) (*UnitState, error)

	MarshalJSON() ([]byte, error)
}
