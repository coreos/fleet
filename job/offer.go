package job

import (
	"sort"
)

type JobOffer struct {
	Job Job

	// MachineBootIDs represents a set of machines for which this offer is valid.
	// If nil or len == 0 then all machines. Must be sorted.
	// MachineIDs started life as MachineBootIds in the datastore.
	// It cannot be changed without a migration
	MachineBootIDs []string `json:"MachineBootIds"`
}

func NewOfferFromJob(j Job, machineBootIDs []string) *JobOffer {
	return &JobOffer{
		Job:            j,
		MachineBootIDs: machineBootIDs,
	}
}

// OfferedTo returns true if job is being offered to specified machine.
func (jo *JobOffer) OfferedTo(machineBootID string) bool {
	// for backward compatibility: if not populated, assume all machines are considered
	if len(jo.MachineBootIDs) == 0 {
		return true
	}

	k := sort.SearchStrings(jo.MachineBootIDs, machineBootID)
	return k < len(jo.MachineBootIDs) && jo.MachineBootIDs[k] == machineBootID
}

type JobBid struct {
	JobName string

	// MachineBootID started life as MachineBootId in the datastore.
	// It cannot be changed without a migration.
	MachineBootID string `json:"MachineBootId"`
}

func NewBid(jobName string, machBootID string) *JobBid {
	return &JobBid{jobName, machBootID}
}
