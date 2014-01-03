package job

import (
	"errors"

	gouuid "github.com/nu7hatch/gouuid"
)

type JobRequest struct {
	ID           gouuid.UUID
	Payloads     []JobPayload
	Peers        []string
	Requirements map[string][]string
	Flags        int
}

func NewJobRequest(payloads []JobPayload, peers []string, requirements map[string][]string) (*JobRequest, error) {
	if payloads == nil || len(payloads) < 1 {
		return nil, errors.New("A minimum of one JobPayload must be provided")
	}

	uuid, err := gouuid.NewV4()
	if err != nil {
		return nil, errors.New("Failed to generate JobRequest.ID")
	}

	return &JobRequest{*uuid, payloads, peers, requirements, 0}, nil
}

func (jr *JobRequest) SetFlag(flag int) {
	jr.Flags |= flag
}

func (jr *JobRequest) IsFlagSet(flag int) bool {
	return (jr.Flags & flag) == flag
}
