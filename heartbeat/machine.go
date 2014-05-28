package heartbeat

import (
	"time"

	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
)

// MachineHeartbeatFunc builds a function that can be used to update
// the given Registry with the current state of the given machine.
func MachineHeartbeatFunc(reg registry.Registry, mach machine.Machine, ttl time.Duration) func() error {
	return func() error {
		_, err := reg.SetMachineState(mach.State(), ttl)
		return err
	}
}
