package job

import (
	"errors"

	gouuid "github.com/nu7hatch/gouuid"

	"github.com/coreos/coreinit/machine"
)

type JobRequest struct {
	ID       gouuid.UUID
	Payloads []JobPayload
	Machines []machine.Machine
}

func NewJobRequest(payloads []JobPayload, machines []machine.Machine) (*JobRequest, error) {
	if payloads == nil || len(payloads) < 1 {
		return nil, errors.New("A minimum of one JobPayload must be provided")
	}

	uuid, err := gouuid.NewV4()
	if err != nil {
		return nil, errors.New("Failed to generate JobRequest.ID")
	}

	return &JobRequest{*uuid, payloads, machines}, nil
}
