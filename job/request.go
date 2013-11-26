package job

import (
	"github.com/coreos/coreinit/machine"
)

type JobRequest struct {
	Machines []machine.Machine
	Payloads []JobPayload
}

func NewJobRequest(machines []machine.Machine, payloads []JobPayload) *JobRequest {
	return &JobRequest{machines, payloads}
}
