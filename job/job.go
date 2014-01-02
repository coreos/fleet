package job

import (
	"errors"
	"fmt"
	"strings"
)

type Job struct {
	Name         string
	Type         string
	State        *JobState
	Payload      *JobPayload
	Peers		 []string
	Requirements map[string][]string
}

func NewJob(name string, state *JobState, payload *JobPayload, requirements map[string][]string) (*Job, error) {
	var peers []string
	var payloadType string
	if strings.HasSuffix(name, ".service") {
		payloadType = "systemd-service"
	} else if strings.HasSuffix(name, ".socket") {
		payloadType = "systemd-socket"

		idx := len(name) - 7
		baseName := name[0:idx]
		svc := fmt.Sprintf("%s.%s", baseName, "service")
		peers = append(peers, svc)
	} else {
		return nil, errors.New(fmt.Sprintf("Unrecognized systemd unit %s", name))
	}

	return &Job{name, payloadType, state, payload, peers, requirements}, nil
}

func (self *Job) String() string {
	return fmt.Sprintf("{Name=%s, Type=%s, Peers=%s, Requirements=%s}", self.Name, self.Type, self.Peers, self.Requirements)
}
