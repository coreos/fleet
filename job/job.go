package job

import (
	"fmt"
)

type Job struct {
	Name         string
	State        *JobState
	Payload      *JobPayload
}

func NewJob(name string, state *JobState, payload *JobPayload) *Job {
	return &Job{name, state, payload}
}

func (self *Job) String() string {
	return fmt.Sprintf("{ Name=%s }", self.Name)
}
