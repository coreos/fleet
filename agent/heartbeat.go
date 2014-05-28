package agent

import (
	"time"

	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
)

// AgentHeartbeatFunc builds a function that can be used to update
// the given Registry with the current state of an Agent.
func AgentHeartbeatFunc(reg registry.Registry, mach machine.Machine, a *Agent, ttl time.Duration) func() error {
	return func() error {
		machID := mach.State().ID
		launched := a.state.LaunchedJobs()
		for _, j := range launched {
			go reg.JobHeartbeat(j, machID, ttl)
		}
		return nil
	}
}
