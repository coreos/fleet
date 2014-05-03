package platform

import (
	"strconv"

	"github.com/coreos/fleet/functional/util"
)

type Cluster interface {
	CreateMember(string, MachineConfig) error
	DestroyMember(string) error
	Members() []string
	MemberCommand(string, ...string) (string, error)
	Destroy() error

	// client operations
	Fleetctl(args ...string) (string, string, error)
	FleetctlWithInput(input string, args ...string) (string, string, error)
	WaitForNActiveUnits(count int) (map[string]util.UnitState, error)
	WaitForNMachines(count int) ([]string, error)
}

// MachineConfig defines the parameters that should
// be considered when creating a new cluster member.
type MachineConfig struct {
	VerifyUnits bool
}

func CreateNClusterMembers(cl Cluster, count int, cfg MachineConfig) error {
	for i := 0; i < count; i++ {
		name := strconv.Itoa(i)
		if err := cl.CreateMember(name, cfg); err != nil {
			return err
		}
	}
	return nil
}
