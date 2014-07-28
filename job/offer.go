package job

type JobOffer struct {
	Job Job

	// This field is ignored, but kept around because it is part of the
	// legacy datastructure stored in the Registry.
	MachineIDs []string
}

func NewOfferFromJob(j Job) *JobOffer {
	return &JobOffer{
		Job:        j,
		MachineIDs: nil,
	}
}
