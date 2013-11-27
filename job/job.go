package job

import (
	"errors"
	"fmt"
	"strings"
)

type Job struct {
	Name    string      `json:"name"`
	Type    string      `json:"type"`
	State   *JobState   `json:"state"`
	Payload *JobPayload `json:"payload"`
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

func NewJobsFromRequest(req *JobRequest) ([]Job, error) {
	jobs := []Job{}
	for i := 0; i < len(req.Payloads); i++ {
		payload := req.Payloads[i]
		job, err := NewJob(payload.Name, nil, &payload)
		if err != nil {
			return nil, err
		} else {
			jobs = append(jobs, *job)
		}
	}
	return jobs, nil
}
