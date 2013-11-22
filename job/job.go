package job

import (
	"errors"
	"fmt"
	"strings"

	"github.com/coreos/coreinit/machine"
)

type Job struct {
	Name    string      `json:"name"`
	Type	string		`json:"type"`
	State   *JobState   `json:"state"`
	Payload *JobPayload `json:"payload"`
}

type JobPayload struct {
	Value string	`json:"value"`
}

func NewJob(name string, state *JobState, payload *JobPayload) (*Job, error) {
	var payloadType string
    if strings.HasSuffix(name, ".service") {
        payloadType = "systemd-service"
    } else if strings.HasSuffix(name, ".socket") {
        payloadType = "systemd-socket"
    } else {
        return nil, errors.New(fmt.Sprintf("Unrecognized systemd unit %s", name))
    }

	return &Job{name, payloadType, state, payload}, nil
}

type JobState struct {
	State   string					`json:"state"`
	Sockets []string				`json:"sockets"`
	Machine *machine.Machine		`json:"machine"`
}

func NewJobState(state string, sockets []string, machine *machine.Machine) *JobState {
	return &JobState{state, sockets, machine}
}
