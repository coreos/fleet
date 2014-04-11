package job

type Job struct {
	Name    string
	Payload JobPayload
	State   *JobState
}

func NewJob(name string, payload JobPayload) *Job {
	return &Job{name, payload, nil}
}

func (self *Job) Requirements() map[string][]string {
	return self.Payload.Requirements()
}
