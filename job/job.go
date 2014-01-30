package job

import (
	"fmt"
)

type Job struct {
	Name         string
	requirements map[string][]string
	Payload      *JobPayload
	State        *JobState
}

func NewJob(name string, requirements map[string][]string, payload *JobPayload, state *JobState) *Job {
	return &Job{name, requirements, payload, state}
}

func (self *Job) String() string {
	return fmt.Sprintf("{ Name=%s }", self.Name)
}

func (self *Job) Requirements() map[string][]string {
	if self.Payload != nil {
		stacked := make(map[string][]string, 0)

		for key, values := range self.Payload.Unit.Requirements() {
			stacked[key] = values
		}

		for key, values := range self.requirements {
			stacked[key] = values
		}

		return stacked

	} else {
		return self.requirements
	}
}
