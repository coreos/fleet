package registry

import (
	"errors"
	"path"
	"strings"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/etcd"
	"github.com/coreos/fleet/event"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/unit"
)

const (
	jobPrefix = "job"
)

// Jobs lists all Jobs known by the Registry, ordered by job name
func (r *EtcdRegistry) Jobs() ([]job.Job, error) {
	var jobs []job.Job

	req := etcd.Get{
		Key:       path.Join(r.keyPrefix, jobPrefix),
		Sorted:    true,
		Recursive: true,
	}

	resp, err := r.etcd.Do(&req)
	if err != nil {
		if isKeyNotFound(err) {
			err = nil
		}
		return jobs, err
	}

	for _, dir := range resp.Node.Nodes {
		for _, node := range dir.Nodes {
			if !strings.HasSuffix(node.Key, "object") {
				continue
			}

			j, err := r.getJobFromJSON(node.Value)
			if j == nil || err != nil {
				log.Infof("Unable to parse Job in Registry at key %s", node.Key)
				continue
			}

			if err = r.hydrateJob(j); err != nil {
				log.Errorf("Failed to hydrate Job(%s) model", j.Name)
				continue
			}

			jobs = append(jobs, *j)
		}
	}

	return jobs, nil
}

// jobTargetMachine looks up where the given job is scheduled. If the job has
// been scheduled, the ID the target machine is returned. Otherwise, an
// empty string is returned.
func (r *EtcdRegistry) jobTargetMachine(jobName string) (string, error) {
	req := etcd.Get{
		Key:       r.jobTargetAgentPath(jobName),
		Sorted:    false,
		Recursive: true,
	}

	resp, err := r.etcd.Do(&req)
	if err != nil {
		if isKeyNotFound(err) {
			err = nil
		}
		return "", err
	}

	return resp.Node.Value, nil
}

func (r *EtcdRegistry) ClearJobTarget(jobName, machID string) error {
	req := etcd.Delete{
		Key:           r.jobTargetAgentPath(jobName),
		PreviousValue: machID,
	}

	_, err := r.etcd.Do(&req)
	if isKeyNotFound(err) {
		err = nil
	}

	return err
}

// Job looks for a Job of the given name in the Registry. It returns a fully
// hydrated Job on success, or nil on any kind of failure.
func (r *EtcdRegistry) Job(jobName string) (*job.Job, error) {
	req := etcd.Get{
		Key: path.Join(r.keyPrefix, jobPrefix, jobName, "object"),
	}

	resp, err := r.etcd.Do(&req)
	if err != nil {
		if isKeyNotFound(err) {
			err = nil
		}
		return nil, err
	}

	var j *job.Job
	j, err = r.getJobFromJSON(resp.Node.Value)
	if j == nil || err != nil {
		return nil, err
	}

	if err = r.hydrateJob(j); err != nil {
		return nil, err
	}

	return j, nil
}

func (r *EtcdRegistry) hydrateJob(j *job.Job) error {
	tgt, err := r.jobTargetState(j.Name)
	if err != nil {
		return err
	}

	j.TargetState = tgt

	j.TargetMachineID, err = r.jobTargetMachine(j.Name)
	if err != nil {
		return err
	}

	j.UnitState = r.getUnitState(j.Name)
	j.State = r.determineJobState(j.Name)

	return nil
}

func (r *EtcdRegistry) getJobFromJSON(val string) (*job.Job, error) {
	var jm jobModel
	if err := unmarshal(val, &jm); err != nil {
		return nil, err
	}

	return r.getJobFromModel(jm), nil
}

func (r *EtcdRegistry) getJobFromModel(jm jobModel) *job.Job {
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
	} else {
		// Old-style Jobs had "Payloads" instead of Units, also stored separately in the Registry
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

	return job.NewJob(jm.Name, *unit)
}

// jobModel is used for serializing and deserializing Jobs stored in the Registry
type jobModel struct {
	Name     string
	UnitHash unit.Hash
}

// DestroyJob removes a Job object from the repository, along with any legacy
// associated Payload and SignatureSet. It does not yet remove underlying
// Units from the repository.
func (r *EtcdRegistry) DestroyJob(jobName string) error {
	req := etcd.Delete{
		Key:       path.Join(r.keyPrefix, jobPrefix, jobName),
		Recursive: true,
	}

	r.etcd.Do(&req)

	// TODO(jonboulle): add unit reference counting and actually destroying Units
	r.destroyLegacyPayload(jobName)
	r.destroySignatureSetOfJob(jobName)
	// TODO(jonboulle): handle errors

	return nil
}

// destroyLegacyPayload removes an old-style Payload from the registry
func (r *EtcdRegistry) destroyLegacyPayload(payloadName string) {
	req := etcd.Delete{
		Key: path.Join(r.keyPrefix, payloadPrefix, payloadName),
	}
	r.etcd.Do(&req)
}

// CreateJob attempts to store a Job and its associated Unit in the registry
func (r *EtcdRegistry) CreateJob(j *job.Job) (err error) {
	if err := r.storeOrGetUnit(j.Unit); err != nil {
		return err
	}

	jm := jobModel{
		Name:     j.Name,
		UnitHash: j.Unit.Hash(),
	}
	json, err := marshal(jm)
	if err != nil {
		return
	}

	req := etcd.Create{
		Key:   path.Join(r.keyPrefix, jobPrefix, j.Name, "object"),
		Value: json,
	}

	_, err = r.etcd.Do(&req)
	if err != nil && isNodeExist(err) {
		err = errors.New("job already exists")
	}

	return
}

func (r *EtcdRegistry) jobTargetState(jobName string) (job.JobState, error) {
	req := etcd.Get{
		Key: r.jobTargetStatePath(jobName),
	}
	resp, err := r.etcd.Do(&req)
	if err != nil {
		if isKeyNotFound(err) {
			err = nil
		}
		return job.JobStateInactive, err
	}

	return job.ParseJobState(resp.Node.Value)
}

func (r *EtcdRegistry) SetJobTargetState(jobName string, state job.JobState) error {
	req := etcd.Set{
		Key:   r.jobTargetStatePath(jobName),
		Value: string(state),
	}
	_, err := r.etcd.Do(&req)
	return err
}

func (es *EventStream) filterJobTargetStateChanges(resp *etcd.Result) *event.Event {
	if resp.Action != "set" {
		return nil
	}

	dir, baseName := path.Split(resp.Node.Key)
	if baseName != "target-state" {
		return nil
	}

	dir = strings.TrimSuffix(dir, "/")
	jobName := path.Base(dir)

	ts, err := job.ParseJobState(resp.Node.Value)
	if err != nil {
		log.Errorf("Failed parsing JobState: %v", err)
		return nil
	}

	cs := es.registry.determineJobState(jobName)
	if cs == nil {
		return nil
	}
	if *cs == ts {
		// No state change has actually occurred
		return nil
	}

	var cType string
	switch {
	case *cs == job.JobStateLoaded && ts == job.JobStateLaunched:
		cType = "JobTargetStateStarted"
	case *cs == job.JobStateLaunched && ts == job.JobStateLoaded:
		cType = "JobTargetStateStopped"
	default:
		cType = "JobTargetStateChange"
	}

	agent, err := es.registry.jobTargetMachine(jobName)
	if err != nil {
		log.Errorf("Failed to look up target of Job(%s): %v", jobName, err)
		return nil
	}

	return &event.Event{cType, jobName, agent}
}

func (r *EtcdRegistry) ScheduleJob(jobName string, machID string) error {
	req := etcd.Create{
		Key:   r.jobTargetAgentPath(jobName),
		Value: machID,
	}
	_, err := r.etcd.Do(&req)
	return err
}

func filterEventJobScheduled(resp *etcd.Result) *event.Event {
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

func filterEventJobUnscheduled(resp *etcd.Result) *event.Event {
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

func filterEventJobDestroyed(resp *etcd.Result) *event.Event {
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

func (r *EtcdRegistry) jobTargetAgentPath(jobName string) string {
	return path.Join(r.keyPrefix, jobPrefix, jobName, "target")
}

func (r *EtcdRegistry) jobTargetStatePath(jobName string) string {
	return path.Join(r.keyPrefix, jobPrefix, jobName, "target-state")
}
