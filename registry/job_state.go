package registry

import (
	"path"
	"time"

	"github.com/coreos/fleet/etcd"
	"github.com/coreos/fleet/job"
)

// determineJobState decides what the State field of a Job object should
// be. The value of heartbeat should be the machine ID that is known to
// have recently heartbeaten (see JobHeartbeat) the Job. All fields of the
// Job (except for State) must be available - no partial representations.
func determineJobState(j *job.Job, heartbeat string) (state job.JobState) {
	state = job.JobStateInactive

	if j.TargetMachineID == "" || j.UnitState == nil {
		return
	}

	state = job.JobStateLoaded

	if heartbeat != j.TargetMachineID {
		return
	}

	state = job.JobStateLaunched
	return
}

func (r *EtcdRegistry) JobHeartbeat(jobName, agentMachID string, ttl time.Duration) error {
	req := etcd.Set{
		Key:   r.jobHeartbeatPath(jobName),
		Value: agentMachID,
		TTL:   ttl,
	}
	_, err := r.etcd.Do(&req)
	return err
}

func (r *EtcdRegistry) ClearJobHeartbeat(jobName string) {
	req := etcd.Delete{
		Key: r.jobHeartbeatPath(jobName),
	}
	r.etcd.Do(&req)
}

func (r *EtcdRegistry) jobHeartbeatPath(jobName string) string {
	return path.Join(r.keyPrefix, jobPrefix, jobName, "job-state")
}
