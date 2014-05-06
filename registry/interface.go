package registry

import (
	"time"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/sign"
	"github.com/coreos/fleet/unit"
)

type Registry interface {
	CheckJobPulse(jobName string) (string, bool)
	ClearJobHeartbeat(jobName string)
	ClearJobTarget(jobName, machID string) error
	CreateJob(j *job.Job) (err error)
	CreateJobOffer(jo *job.JobOffer) error
	CreateSignatureSet(ss *sign.SignatureSet) error
	DestroyJob(jobName string)
	DestroySignatureSet(tag string)
	GetActiveMachines() []machine.MachineState
	GetAllJobs() []job.Job
	GetDebugInfo() (string, error)
	GetJob(jobName string) (j *job.Job)
	GetJobTarget(jobName string) string
	GetJobTargetState(jobName string) *job.JobState
	GetMachineState(machID string) *machine.MachineState
	GetSignatureSetOfJob(name string) *sign.SignatureSet
	GetSignatureSet(tag string) *sign.SignatureSet
	JobHeartbeat(jobName, agentMachID string, ttl time.Duration) error
	LockJob(jobName, context string) *TimedResourceMutex
	LockJobOffer(jobName, context string) *TimedResourceMutex
	LockMachine(machID, context string) *TimedResourceMutex
	RemoveMachineState(machID string) error
	RemoveUnitState(jobName string) error
	ResolveJobOffer(jobName string) error
	SaveUnitState(jobName string, unitState *unit.UnitState)
	ScheduleJob(jobName string, machID string) error
	SetJobTargetState(jobName string, state job.JobState) error
	SetMachineState(ms machine.MachineState, ttl time.Duration) (uint64, error)
	SubmitJobBid(jb *job.JobBid)
	UnresolvedJobOffers() []job.JobOffer
}
