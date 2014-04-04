package machine

import (
	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/version"
)

// Machine provides the means for a caller to access a relatively
// up-to-date MachineState object.
type Machine struct {
	staticState  MachineState
	dynamicState *MachineState
}

// New creates a new Machine object. The provided parameters will override
// those that might be dynamically generated by the Machine on the fly.
func New(bootID string, publicIP string, metadata map[string]string) *Machine {
	static := MachineState{bootID, publicIP, metadata, version.Version}
	log.V(2).Infof("Created Machine with static state %s", static)
	m := &Machine{staticState: static}
	return m
}

func (m *Machine) String() string {
	return m.State().BootID
}

// State returns a MachineState object representing the Machine's
// static state overlaid on its dynamic state at the time of execution.
func (m *Machine) State() (state MachineState) {
	if m.dynamicState == nil {
		state = MachineState(m.staticState)
	} else {
		state = stackState(m.staticState, *m.dynamicState)
	}

	return
}

// RefreshState generates a new MachineState object based on the
// current state of the underlying host, storing it internally for
// future reference before returning it.
func (m *Machine) RefreshState() *MachineState {
	state := CurrentState()
	m.dynamicState = &state
	return &state
}

// HasMetadata determine if a Machine fulfills the given requirements
// based on its current state.
func (m *Machine) HasMetadata(metadata map[string][]string) bool {
	state := m.State()

	for key, values := range metadata {
		local, ok := state.Metadata[key]
		if !ok {
			log.V(1).Infof("No local values found for Metadata(%s)", key)
			return false
		}

		log.V(2).Infof("Asserting local Metadata(%s) meets requirements", key)

		var localMatch bool
		for _, val := range values {
			if local == val {
				log.V(1).Infof("Local Metadata(%s) meets requirement", key)
				localMatch = true
			}
		}

		if !localMatch {
			log.V(1).Infof("Local Metadata(%s) does not match requirement", key)
			return false
		}
	}

	return true
}
