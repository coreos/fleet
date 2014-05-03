package registry

import (
	"path"
	"time"

	"github.com/coreos/fleet/job"
)

func (r *Registry) determineJobState(jobName string) *job.JobState {
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

func (r *Registry) JobHeartbeat(jobName, agentMachID string, ttl time.Duration) error {
	key := r.jobHeartbeatPath(jobName)
	_, err := r.etcd.Set(key, agentMachID, uint64(ttl.Seconds()))
	return err
}

func (r *Registry) CheckJobPulse(jobName string) (string, bool) {
	key := r.jobHeartbeatPath(jobName)
	resp, err := r.etcd.Get(key, false, false)
	if err != nil {
		return "", false
	}

	return resp.Node.Value, true
}

func (r *Registry) ClearJobHeartbeat(jobName string) {
	key := r.jobHeartbeatPath(jobName)
	r.etcd.Delete(key, false)
}

func (r *Registry) jobHeartbeatPath(jobName string) string {
	return path.Join(r.keyPrefix, jobPrefix, jobName, "job-state")
}
