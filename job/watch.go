package job

type JobWatch struct {
	Payload *JobPayload
	Count   int
}

func NewJobWatch(j *JobPayload, count int) *JobWatch {
	return &JobWatch{j, count}
}
