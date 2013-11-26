package job

import (
	"github.com/coreos/coreinit/machine"
)

type JobState struct {
	LoadState   string           `json:"loadState"`
	ActiveState string           `json:"activeState"`
	SubState    string           `json:"subState"`
	Sockets     []string         `json:"sockets"`
	Machine     *machine.Machine `json:"machine"`
}

func NewJobState(loadState, activeState, subState string, sockets []string, machine *machine.Machine) *JobState {
	return &JobState{loadState, activeState, subState, sockets, machine}
}
