package job

import (
	"sort"
)

type JobOffer struct {
	Job Job
	// Offer just for the machines in this slice
	// if nil or len == 0 then all machines.
	// Needs to be sorted.
	MachineBootIds []string
}

func NewOfferFromJob(j Job, machineBootIds []string) *JobOffer {
	return &JobOffer{
		Job:            j,
		MachineBootIds: machineBootIds,
	}
}

// OfferedTo returns true if job is being offered to specified machine.
func (jo *JobOffer) OfferedTo(machineBootId string) bool {
	// for backward compatibility: if not populated, assume all machines are considered
	if len(jo.MachineBootIds) == 0 {
		return true
	}

	k := sort.SearchStrings(jo.MachineBootIds, machineBootId)
	return k < len(jo.MachineBootIds) && jo.MachineBootIds[k] == machineBootId
}

type JobBid struct {
	JobName       string
	MachineBootId string
}

func NewBid(jobName string, machBootId string) *JobBid {
	return &JobBid{jobName, machBootId}
}
