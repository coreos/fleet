package job

type Job struct {
	Name            string
	JobRequirements map[string][]string
	Payload         *JobPayload
	State           *JobState
}

func NewJob(name string, requirements map[string][]string, payload *JobPayload, state *JobState) *Job {
	return &Job{name, requirements, payload, state}
}

func (self *Job) Requirements() map[string][]string {
	if self.Payload != nil {
		stacked := make(map[string][]string, 0)

		for key, values := range self.Payload.Requirements() {
			stacked[key] = values
		}

		for key, values := range self.JobRequirements {
			stacked[key] = values
		}

		return stacked

	} else {
		return self.JobRequirements
	}
}
