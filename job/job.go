package job

type JobState string

const (
	JobStateInactive = JobState("inactive")
	JobStateLoaded   = JobState("loaded")
	JobStateLaunched = JobState("launched")
)

func ParseJobState(s string) *JobState {
	js := JobState(s)
	if js != JobStateInactive && js != JobStateLoaded && js != JobStateLaunched {
		return nil
	}
	return &js
}

type Job struct {
	Name         string
	Payload      JobPayload
	State        *JobState
	PayloadState *PayloadState
}

func NewJob(name string, payload JobPayload) *Job {
	return &Job{name, payload, nil, nil}
}

func (self *Job) Requirements() map[string][]string {
	return self.Payload.Requirements()
}
