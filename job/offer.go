package job

type JobOffer struct {
	Job Job
}

func NewOfferFromJob(j Job) *JobOffer {
	return &JobOffer{j}
}

type JobBid struct {
	JobName       string
	MachineBootId string
}

func NewBid(jobName string, machBootId string) *JobBid {
	return &JobBid{jobName, machBootId}
}
