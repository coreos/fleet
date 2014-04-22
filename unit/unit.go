package unit

import (
	"crypto/sha1"
	"fmt"
	"strings"

	"github.com/coreos/fleet/machine"
)

// Fleet specific unit file requirement keys.
// "X-" prefix only appears in unit file and is dropped in code before the value is used.
const (
	// Require the unit be scheduled to a specific machine defined by given boot ID.
	FleetXConditionMachineBootID = "ConditionMachineBootID"
	// Limit eligible machines to the one that hosts a specific unit.
	FleetXConditionMachineOf = "ConditionMachineOf"
	// Prevent a unit from being collocated with other units using glob-matching on the other unit names.
	FleetXConflicts = "Conflicts"
)

func SupportedUnitTypes() []string {
	return []string{"service", "socket"}
}

// SHA1 sum
type Hash [sha1.Size]byte

func (h Hash) String() string {
	return fmt.Sprintf("%x", h[:])
}

func (h *Hash) Empty() bool {
	return *h == Hash{}
}

// A Unit represents a systemd configuration which encodes information about any of the unit
// types that fleet supports (as defined in SupportedUnitTypes()).
// Units are linked to Jobs by the Hash of their contents.
// Similar to systemd, a Unit configuration has no inherent name, but is rather
// named through the reference to it; in the case of systemd, the reference is
// the filename, and in the case of fleet, the reference is the name of the job
// that references this Unit.
type Unit struct {
	// Contents represents the parsed unit file.
	// This field must be considered readonly.
	Contents map[string]map[string][]string

	// Raw represents the entire contents of the unit file.
	raw string
}

func (self *Unit) String() string {
	return self.raw
}

// Hash returns the SHA1 hash of the raw contents of the Unit
func (u *Unit) Hash() Hash {
	return Hash(sha1.Sum([]byte(u.raw)))
}

// Requirements returns all relevant options from the [X-Fleet] section of a unit file.
// Relevant options are identified with a `X-` prefix in the unit.
// This prefix is stripped from relevant options before being returned.
func (u *Unit) Requirements() map[string][]string {
	requirements := make(map[string][]string)
	for key, value := range u.Contents["X-Fleet"] {
		if !strings.HasPrefix(key, "X-") {
			continue
		}

		// Strip off leading X-
		key = key[2:]

		if _, ok := requirements[key]; !ok {
			requirements[key] = make([]string, 0)
		}

		requirements[key] = value
	}

	return requirements
}

func (u *Unit) Conflicts() []string {
	conflicts, ok := u.Requirements()[FleetXConflicts]
	if ok {
		return conflicts
	} else {
		return make([]string, 0)
	}
}

// UnitState encodes the current state of a unit loaded into systemd
type UnitState struct {
	LoadState    string                `json:"loadState"`
	ActiveState  string                `json:"activeState"`
	SubState     string                `json:"subState"`
	Sockets      []string              `json:"sockets"`
	MachineState *machine.MachineState `json:"machineState"`
}

func NewUnitState(loadState, activeState, subState string, sockets []string, ms *machine.MachineState) *UnitState {
	return &UnitState{loadState, activeState, subState, sockets, ms}
}
