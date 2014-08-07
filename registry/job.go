package registry

import (
	"errors"
	"path"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/etcd"
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
		objKey := path.Join(dir.Key, "object")
		var obj *etcd.Node
		for _, node := range dir.Nodes {
			if node.Key != objKey {
				continue
			}
			node := node
			obj = &node
		}

		if obj == nil {
			continue
		}

		j, err := r.getJobFromObjectNode(obj)
		if j == nil || err != nil {
			log.Infof("Unable to parse Job in Registry at key %s", obj.Key)
			continue
		}

		if err = r.parseJobDir(j, &dir); err != nil {
			log.Errorf("Failed to parse Job(%s) model: %v", j.Name, err)
			continue
		}

		jobs = append(jobs, *j)
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
		Key:       path.Join(r.keyPrefix, jobPrefix, jobName),
		Recursive: true,
	}

	resp, err := r.etcd.Do(&req)
	if err != nil {
		if isKeyNotFound(err) {
			err = nil
		}
		return nil, err
	}

	objKey := path.Join(req.Key, "object")
	var obj *etcd.Node
	for _, node := range resp.Node.Nodes {
		if node.Key != objKey {
			continue
		}
		node := node
		obj = &node
	}

	if obj == nil {
		return nil, nil
	}

	j, err := r.getJobFromObjectNode(obj)
	if j == nil || err != nil {
		return nil, err
	}

	if err = r.parseJobDir(j, resp.Node); err != nil {
		return nil, err
	}

	return j, nil
}

func (r *EtcdRegistry) parseJobDir(j *job.Job, dir *etcd.Node) (err error) {
	var heartbeat string
	for _, node := range dir.Nodes {
		switch node.Key {
		case r.jobTargetStatePath(j.Name):
			j.TargetState, err = job.ParseJobState(node.Value)
			if err != nil {
				return
			}
		case r.jobTargetAgentPath(j.Name):
			j.TargetMachineID = node.Value
		case r.jobHeartbeatPath(j.Name):
			heartbeat = node.Value
		}
	}

	j.UnitState = r.getUnitState(j.Name)

	js := determineJobState(j, heartbeat)
	j.State = &js

	return
}

func (r *EtcdRegistry) getJobFromObjectNode(node *etcd.Node) (*job.Job, error) {
	var err error
	var jm jobModel
	if err = unmarshal(node.Value, &jm); err != nil {
		return nil, err
	}

	if jm.UnitHash.Empty() {
		err := fmt.Errorf("Unable to look up unit for Job(%s), job model has no UnitHash field", jm.Name)
		return nil, err
	}
	unit := r.getUnitByHash(jm.UnitHash)
	if unit == nil {
		log.Warningf("No Unit found in Registry for Job(%s)", jm.Name)
		return nil, nil
	}
	if unit.Hash() != jm.UnitHash {
		log.Errorf("Unit Hash %s does not match expected %s for Job(%s)!", unit.Hash(), jm.UnitHash, jm.Name)
		return nil, nil
	}

	return job.NewJob(jm.Name, *unit), nil
}

// jobModel is used for serializing and deserializing Jobs stored in the Registry
type jobModel struct {
	Name     string
	UnitHash unit.Hash
}

// DestroyJob removes a Job object from the repository along with any SignatureSets.
// It does not yet remove underlying Units from the repository.
func (r *EtcdRegistry) DestroyJob(jobName string) error {
	req := etcd.Delete{
		Key:       path.Join(r.keyPrefix, jobPrefix, jobName),
		Recursive: true,
	}

	_, err := r.etcd.Do(&req)
	if err != nil {
		if isKeyNotFound(err) {
			err = errors.New("job does not exist")
		}

		return err
	}

	// TODO(jonboulle): add unit reference counting and actually destroying Units

	r.destroySignatureSetOfJob(jobName)
	// TODO(jonboulle): handle errors

	return nil
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

func (r *EtcdRegistry) updateJobObjectNode(jm *jobModel, idx uint64) (err error) {
	json, err := marshal(jm)
	if err != nil {
		return
	}

	req := etcd.Set{
		Key:           path.Join(r.keyPrefix, jobPrefix, jm.Name, "object"),
		Value:         json,
		PreviousIndex: idx,
	}

	_, err = r.etcd.Do(&req)
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

func (r *EtcdRegistry) ScheduleJob(jobName string, machID string) error {
	req := etcd.Create{
		Key:   r.jobTargetAgentPath(jobName),
		Value: machID,
	}
	_, err := r.etcd.Do(&req)
	return err
}

func (r *EtcdRegistry) jobTargetAgentPath(jobName string) string {
	return path.Join(r.keyPrefix, jobPrefix, jobName, "target")
}

func (r *EtcdRegistry) jobTargetStatePath(jobName string) string {
	return path.Join(r.keyPrefix, jobPrefix, jobName, "target-state")
}
