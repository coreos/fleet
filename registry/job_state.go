package registry

import (
	"path"
	"time"

	"github.com/coreos/fleet/job"
)

func (r *FleetRegistry) determineJobState(jobName string) *job.JobState {
	state := job.JobStateInactive

	tgt := r.GetJobTarget(jobName)
	if tgt == "" {
		return &state
	}

	if r.getUnitState(jobName) == nil {
		return &state
	}

	state = job.JobStateLoaded

	agent, pulse := r.CheckJobPulse(jobName)
	if !pulse || agent != tgt {
		return &state
	}

	state = job.JobStateLaunched
	return &state
}

func (r *FleetRegistry) JobHeartbeat(jobName, agentMachID string, ttl time.Duration) error {
	key := r.jobHeartbeatPath(jobName)
	_, err := r.storage.Set(key, agentMachID, uint64(ttl.Seconds()))
	return err
}

func (r *FleetRegistry) CheckJobPulse(jobName string) (string, bool) {
	key := r.jobHeartbeatPath(jobName)
	resp, err := r.storage.Get(key, false, false)
	if err != nil {
		return "", false
	}

	return resp.Node.Value, true
}

func (r *FleetRegistry) ClearJobHeartbeat(jobName string) {
	key := r.jobHeartbeatPath(jobName)
	r.storage.Delete(key, false)
}

func (r *FleetRegistry) jobHeartbeatPath(jobName string) string {
	return path.Join(r.keyPrefix, jobPrefix, jobName, "job-state")
}
