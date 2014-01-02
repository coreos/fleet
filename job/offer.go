package job

type JobOffer struct {
	Job          Job
}

func NewOfferFromJob(j Job) *JobOffer {
	return &JobOffer{j}
}

type JobBid struct {
	JobName     string
	MachineName string
}

func NewBid(jobName string, machName string) *JobBid {
	return &JobBid{jobName, machName}
}
