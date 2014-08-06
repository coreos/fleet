package machine

import "github.com/coreos/fleet/resource"

const (
	shortIDLen = 8
)

// MachineState represents a point-in-time snapshot of the
// state of the local host.
type MachineState struct {
	ID       string
	PublicIP string
	Metadata map[string]string
	Version  string

	// The total resources available on the underlying system
	TotalResources resource.ResourceTuple

	// The resoures considered available for scheduling by fleet
	FreeResources resource.ResourceTuple
}

// UpdateFreeResources populates the FreeResources of a MachineState, given a
// map of units to resource reservations, using the following formula:
// FreeResources = TotalResources - (sum(unit resource reservations) + HostResources)
func UpdateFreeResources(ms MachineState, reservations map[string]resource.ResourceTuple) MachineState {
	all := []resource.ResourceTuple{resource.HostResources}
	for _, res := range reservations {
		all = append(all, res)
	}
	reserved := resource.Sum(all...)
	// TODO(jonboulle): check for negatives!
	ms.FreeResources = resource.Sub(ms.TotalResources, reserved)
	return ms
}

func (ms MachineState) ShortID() string {
	if len(ms.ID) <= shortIDLen {
		return ms.ID
	}
	return ms.ID[0:shortIDLen]
}

func (ms MachineState) MatchID(ID string) bool {
	return ms.ID == ID || ms.ShortID() == ID
}

// stackState is used to merge two MachineStates. Values configured on the top
// MachineState always take precedence over those on the bottom.
func stackState(top, bottom MachineState) MachineState {
	state := MachineState(bottom)

	if top.PublicIP != "" {
		state.PublicIP = top.PublicIP
	}

	if top.ID != "" {
		state.ID = top.ID
	}

	if top.TotalResources.Cores > 0 {
		state.TotalResources.Cores = top.TotalResources.Cores
	}

	if top.TotalResources.Memory > 0 {
		state.TotalResources.Memory = top.TotalResources.Memory
	}

	if top.TotalResources.Disk > 0 {
		state.TotalResources.Disk = top.TotalResources.Disk
	}

	//FIXME: This will *always* overwrite the bottom's metadata,
	// but the only use-case we have today does not ever have
	// metadata on the bottom.
	if len(top.Metadata) > 0 {
		state.Metadata = top.Metadata
	}

	if top.Version != "" {
		state.Version = top.Version
	}

	return state
}
