package registry

import (
	"errors"
	"path"
	"strings"

	etcdErr "github.com/coreos/fleet/third_party/github.com/coreos/etcd/error"
	"github.com/coreos/fleet/third_party/github.com/coreos/go-etcd/etcd"
	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/event"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/unit"
)

const (
	jobPrefix = "job"
)

// GetAllJobs lists all Jobs known by the Registry
func (r *FleetRegistry) GetAllJobs() []job.Job {
	var jobs []job.Job

	key := path.Join(r.keyPrefix, jobPrefix)
	resp, err := r.storage.Get(key, true, true)

	if err != nil {
		return jobs
	}

	for _, dir := range resp.Node.Nodes {
		for _, node := range dir.Nodes {
			if !strings.HasSuffix(node.Key, "object") {
				continue
			}

			j := r.getJobFromJSON(node.Value)
			if j == nil {
				continue
			}

			jobs = append(jobs, *j)
		}
	}

	return jobs
}

// GetJobTarget looks up where the given job is scheduled. If the job has
// been scheduled, the ID the target machine is returned. Otherwise, an
// empty string is returned.
func (r *FleetRegistry) GetJobTarget(jobName string) string {
	// Figure out to which Machine this Job is scheduled
	key := r.jobTargetAgentPath(jobName)
	resp, err := r.storage.Get(key, false, true)
	if err != nil {
		return ""
	}

	return resp.Node.Value
}

func (r *FleetRegistry) ClearJobTarget(jobName, machID string) error {
	key := r.jobTargetAgentPath(jobName)
	_, err := r.storage.CompareAndDelete(key, machID, 0)
	return err
}

// GetJob looks for a Job of the given name in the Registry. It returns a fully
// hydrated Job on success, or nil on any kind of failure.
func (r *FleetRegistry) GetJob(jobName string) (j *job.Job) {
	key := path.Join(r.keyPrefix, jobPrefix, jobName, "object")
	resp, err := r.storage.Get(key, false, true)

	// Assume the error was KeyNotFound and return an empty data structure
	if err != nil {
		return nil
	}

	j = r.getJobFromJSON(resp.Node.Value)
	if j == nil {
		log.V(1).Infof("Error unmarshaling Job(%s): %v", jobName, err)
	}
	return
}

func (r *FleetRegistry) getJobFromJSON(val string) *job.Job {
	var jm jobModel
	if err := unmarshal(val, &jm); err != nil {
		return nil
	}

	return r.getJobFromModel(jm)
}

func (r *FleetRegistry) getJobFromModel(jm jobModel) *job.Job {
	var err error
	var unit *unit.Unit

	// New-style Jobs should have a populated UnitHash, and the contents of the Unit are stored separately in the Registry
	if !jm.UnitHash.Empty() {
		unit = r.getUnitByHash(jm.UnitHash)
		if unit == nil {
			log.Warningf("No Unit found in Registry for Job(%s)", jm.Name)
			return nil
		}
		if unit.Hash() != jm.UnitHash {
			log.Errorf("Unit Hash %s does not match expected %s for Job(%s)!", unit.Hash(), jm.UnitHash, jm.Name)
			return nil
		}
		log.V(2).Infof("Got Unit for Job(%s) from registry", jm.Name)
	} else {
		// Old-style Jobs had "Payloads" instead of Units, also stored separately in the Registry
		log.V(2).Infof("Legacy Job(%s) has no PayloadHash - looking for associated Payload", jm.Name)
		unit, err = r.getUnitFromLegacyPayload(jm.Name)
		if err != nil {
			log.Errorf("Error retrieving legacy payload for Job(%s)", jm.Name)
			return nil
		} else if unit == nil {
			log.Warningf("No Payload found in Registry for Job(%s)", jm.Name)
			return nil
		}

		log.Infof("Migrating legacy Payload(%s)", jm.Name)
		if err := r.storeOrGetUnit(*unit); err != nil {
			log.Warningf("Unable to migrate legacy Payload: %v", err)
		}
	}

	j := job.NewJob(jm.Name, *unit)

	j.UnitState = r.getUnitState(jm.Name)
	j.State = r.determineJobState(jm.Name)

	return j
}

// jobModel is used for serializing and deserializing Jobs stored in the Registry
type jobModel struct {
	Name     string
	UnitHash unit.Hash
}

// DestroyJob removes a Job object from the repository, along with any legacy
// associated Payload and SignatureSet. It does not yet remove underlying
// Units from the repository.
func (r *FleetRegistry) DestroyJob(jobName string) {
	key := path.Join(r.keyPrefix, jobPrefix, jobName)
	r.storage.Delete(key, true)
	// TODO(jonboulle): add unit reference counting and actually destroying Units
	r.destroyLegacyPayload(jobName)
	r.destroySignatureSetOfJob(jobName)
}

// destroyLegacyPayload removes an old-style Payload from the registry
func (r *FleetRegistry) destroyLegacyPayload(payloadName string) {
	key := path.Join(r.keyPrefix, payloadPrefix, payloadName)
	r.storage.Delete(key, false)
}

// CreateJob attempts to store a Job and its associated Unit in the registry
func (r *FleetRegistry) CreateJob(j *job.Job) (err error) {
	if err := r.storeOrGetUnit(j.Unit); err != nil {
		return err
	}

	key := path.Join(r.keyPrefix, jobPrefix, j.Name, "object")

	jm := jobModel{
		Name:     j.Name,
		UnitHash: j.UnitHash,
	}
	json, err := marshal(jm)
	if err != nil {
		return
	}

	_, err = r.storage.Create(key, json, 0)
	if err != nil && err.(*etcd.EtcdError).ErrorCode == etcdErr.EcodeNodeExist {
		err = errors.New("job already exists")
	}

	return
}

func (r *FleetRegistry) GetJobTargetState(jobName string) *job.JobState {
	key := r.jobTargetStatePath(jobName)
	resp, err := r.storage.Get(key, false, false)
	if err != nil {
		if err.(*etcd.EtcdError).ErrorCode != etcdErr.EcodeNodeExist {
			log.Errorf("Unable to determine target-state of Job(%s): %v", jobName, err)
		}
		return nil
	}

	return job.ParseJobState(resp.Node.Value)
}

func (r *FleetRegistry) SetJobTargetState(jobName string, state job.JobState) error {
	key := r.jobTargetStatePath(jobName)
	_, err := r.storage.Set(key, string(state), 0)
	return err
}

func (es *EventStream) filterJobTargetStateChanges(resp *etcd.Response) *event.Event {
	if resp.Action != "set" {
		return nil
	}

	dir, baseName := path.Split(resp.Node.Key)
	if baseName != "target-state" {
		return nil
	}

	dir = strings.TrimSuffix(dir, "/")
	jobName := path.Base(dir)

	ts := job.ParseJobState(resp.Node.Value)
	if ts == nil {
		return nil
	}

	cs := es.registry.determineJobState(jobName)
	if *cs == *ts {
		return nil
	}

	var cType string
	switch *cs {
	case job.JobStateInactive:
		cType = "CommandLoadJob"
	case job.JobStateLoaded:
		if *ts == job.JobStateInactive {
			cType = "CommandUnloadJob"
		} else if *ts == job.JobStateLaunched {
			cType = "CommandStartJob"
		}
	case job.JobStateLaunched:
		if *ts == job.JobStateLoaded {
			cType = "CommandStopJob"
		} else if *ts == job.JobStateInactive {
			cType = "CommandUnloadJob"
		}
	}

	if cType == "" {
		return nil
	}

	agent := es.registry.GetJobTarget(jobName)
	return &event.Event{cType, jobName, agent}
}

func (r *FleetRegistry) ScheduleJob(jobName string, machID string) error {
	key := r.jobTargetAgentPath(jobName)
	_, err := r.storage.Create(key, machID, 0)
	return err
}

func (r *FleetRegistry) LockJob(jobName, context string) *TimedResourceMutex {
	return r.lockResource("job", jobName, context)
}

func filterEventJobScheduled(resp *etcd.Response) *event.Event {
	if resp.Action != "create" {
		return nil
	}

	dir, baseName := path.Split(resp.Node.Key)
	if baseName != "target" {
		return nil
	}

	dir = strings.TrimSuffix(dir, "/")
	dir, jobName := path.Split(dir)

	dir = strings.TrimSuffix(dir, "/")
	dir, prefixName := path.Split(dir)

	if prefixName != jobPrefix {
		return nil
	}

	return &event.Event{"EventJobScheduled", jobName, resp.Node.Value}
}

func filterEventJobUnscheduled(resp *etcd.Response) *event.Event {
	if resp.Action != "delete" && resp.Action != "compareAndDelete" {
		return nil
	}

	dir, baseName := path.Split(resp.Node.Key)
	if baseName != "target" {
		return nil
	}

	dir = strings.TrimSuffix(dir, "/")
	dir, jobName := path.Split(dir)

	dir = strings.TrimSuffix(dir, "/")
	dir, prefixName := path.Split(dir)

	if prefixName != jobPrefix {
		return nil
	}

	if resp.PrevNode == nil {
		return nil
	}

	return &event.Event{"EventJobUnscheduled", jobName, resp.PrevNode.Value}
}

func filterEventJobDestroyed(resp *etcd.Response) *event.Event {
	if resp.Action != "delete" {
		return nil
	}

	dir, jobName := path.Split(resp.Node.Key)
	dir = strings.TrimSuffix(dir, "/")
	dir, prefixName := path.Split(dir)

	if prefixName != jobPrefix {
		return nil
	}

	return &event.Event{"EventJobDestroyed", jobName, nil}
}

func (r *FleetRegistry) jobTargetAgentPath(jobName string) string {
	return path.Join(r.keyPrefix, jobPrefix, jobName, "target")
}

func (r *FleetRegistry) jobTargetStatePath(jobName string) string {
	return path.Join(r.keyPrefix, jobPrefix, jobName, "target-state")
}
