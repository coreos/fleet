package job

import (
	"github.com/coreos/fleet/machine"
)

type JobState struct {
	LoadState    string                `json:"loadState"`
	ActiveState  string                `json:"activeState"`
	SubState     string                `json:"subState"`
	Sockets      []string              `json:"sockets"`
	MachineState *machine.MachineState `json:"machineState"`
}

func NewJobState(loadState, activeState, subState string, sockets []string, ms *machine.MachineState) *JobState {
	return &JobState{loadState, activeState, subState, sockets, ms}
}
