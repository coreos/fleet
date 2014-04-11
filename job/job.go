package job

type Job struct {
	Name         string
	Payload      JobPayload
	PayloadState *PayloadState
}

func NewJob(name string, payload JobPayload) *Job {
	return &Job{name, payload, nil}
}

func (self *Job) Requirements() map[string][]string {
	return self.Payload.Requirements()
}
