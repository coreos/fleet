package registry

import (
	"errors"
	"path"
	"sort"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/etcd"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/unit"
)

const (
	jobPrefix = "job"
)

// Jobs lists all Jobs known by the Registry, ordered by job name
func (r *EtcdRegistry) jobs() ([]job.Job, error) {
	var jobs []job.Job

	req := etcd.Get{
		Key:       path.Join(r.keyPrefix, jobPrefix),
		Sorted:    true,
		Recursive: true,
	}

	res, err := r.etcd.Do(&req)
	if err != nil {
		if isKeyNotFound(err) {
			err = nil
		}
		return jobs, err
	}

	for _, dir := range res.Node.Nodes {
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

		if err = r.hydrateJobFromDir(j, &dir); err != nil {
			log.Errorf("Failed to parse Job(%s) model: %v", j.Name, err)
			continue
		}

		jobs = append(jobs, *j)
	}

	return jobs, nil
}

// Schedule returns all ScheduledUnits known by fleet, ordered by name
func (r *EtcdRegistry) Schedule() ([]job.ScheduledUnit, error) {
	req := etcd.Get{
		Key:       path.Join(r.keyPrefix, jobPrefix),
		Sorted:    true,
		Recursive: true,
	}

	res, err := r.etcd.Do(&req)
	if err != nil {
		if isKeyNotFound(err) {
			err = nil
		}
		return nil, err
	}

	heartbeats := make(map[string]string)
	units := make(map[string]*job.ScheduledUnit)

	for _, dir := range res.Node.Nodes {
		_, name := path.Split(dir.Key)
		j := &job.ScheduledUnit{
			Name: name,
		}
		heartbeat, _, err := r.parseJobDir(j, &dir)
		if err != nil {
			log.Errorf("Failed to parse Job(%s) model: %v", j.Name, err)
			continue
		}
		heartbeats[name] = heartbeat
		units[name] = j
	}

	states, err := r.statesByMUSKey()
	if err != nil {
		return nil, err
	}

	var sortable sort.StringSlice

	// Determine the JobState of each ScheduledUnit
	for jName, job := range units {
		sortable = append(sortable, jName)
		key := MUSKey{
			machID: job.TargetMachineID,
			name:   jName,
		}
		us := states[key]
		js := determineJobState(heartbeats[jName], job.TargetMachineID, us)
		job.State = &js
	}
	sortable.Sort()

	var sortedJobs []job.ScheduledUnit
	for _, jName := range sortable {
		sortedJobs = append(sortedJobs, *units[jName])
	}
	return sortedJobs, nil
}

// Units lists all Units known by the Registry, ordered by job name
func (r *EtcdRegistry) Units() ([]job.Unit, error) {
	jobs, err := r.jobs()
	if err != nil {
		return nil, err
	}
	units := make([]job.Unit, len(jobs))
	for i, j := range jobs {
		units[i] = job.Unit{
			Name:        j.Name,
			Unit:        j.Unit,
			TargetState: j.TargetState,
		}
	}
	return units, nil
}

func (r *EtcdRegistry) Unit(name string) (*job.Unit, error) {
	j, err := r.job(name)
	if err != nil || j == nil {
		return nil, err
	}
	u := job.Unit{
		Name:        j.Name,
		Unit:        j.Unit,
		TargetState: j.TargetState,
	}
	return &u, nil
}

func (r *EtcdRegistry) ScheduledUnit(name string) (*job.ScheduledUnit, error) {
	j, err := r.job(name)
	if err != nil || j == nil {
		return nil, err
	}
	su := job.ScheduledUnit{
		Name:            j.Name,
		State:           j.State,
		TargetMachineID: j.TargetMachineID,
	}
	return &su, nil
}

func (r *EtcdRegistry) UnscheduleUnit(name, machID string) error {
	req := etcd.Delete{
		Key:           r.jobTargetAgentPath(name),
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
func (r *EtcdRegistry) job(jobName string) (*job.Job, error) {
	req := etcd.Get{
		Key:       path.Join(r.keyPrefix, jobPrefix, jobName),
		Recursive: true,
	}

	res, err := r.etcd.Do(&req)
	if err != nil {
		if isKeyNotFound(err) {
			err = nil
		}
		return nil, err
	}

	objKey := path.Join(req.Key, "object")
	var obj *etcd.Node
	for _, node := range res.Node.Nodes {
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

	if err = r.hydrateJobFromDir(j, res.Node); err != nil {
		return nil, err
	}

	return j, nil
}

// hydrateJobFromDir fully hydrates a legacy Job struct
func (r *EtcdRegistry) hydrateJobFromDir(j *job.Job, dir *etcd.Node) (err error) {
	var heartbeat string
	var tgtstate job.JobState
	su := &job.ScheduledUnit{
		Name: j.Name,
	}
	if heartbeat, tgtstate, err = r.parseJobDir(su, dir); err != nil {
		return
	}
	j.TargetMachineID = su.TargetMachineID
	j.TargetState = tgtstate

	j.UnitState = r.getUnitState(j.Name)

	js := determineJobState(heartbeat, j.TargetMachineID, j.UnitState)
	j.State = &js

	return
}

// parseJobDir parses an etcd node containing a job, and populates the
// TargetState and TargetMachineID fields of the job. It returns a string
// representing the machine ID that has recently heartbeaten the job (or a nil
// string if none is found) and any error encountered.
func (r *EtcdRegistry) parseJobDir(su *job.ScheduledUnit, dir *etcd.Node) (heartbeat string, tgtstate job.JobState, err error) {
	for _, node := range dir.Nodes {
		switch node.Key {
		case r.jobTargetStatePath(su.Name):
			tgtstate, err = job.ParseJobState(node.Value)
			if err != nil {
				return
			}
		case r.jobTargetAgentPath(su.Name):
			su.TargetMachineID = node.Value
		case r.jobHeartbeatPath(su.Name):
			heartbeat = node.Value
		}
	}

	return heartbeat, tgtstate, err
}

func (r *EtcdRegistry) getUnitFromObjectNode(node *etcd.Node) (*job.Unit, error) {
	j, err := r.getJobFromObjectNode(node)
	if err != nil {
		return nil, err
	}
	ju := &job.Unit{
		Name: j.Name,
		Unit: j.Unit,
	}
	return ju, nil
}

func (r *EtcdRegistry) getJobFromObjectNode(node *etcd.Node) (*job.Job, error) {
	var err error
	var jm jobModel
	if err = unmarshal(node.Value, &jm); err != nil {
		return nil, err
	}

	var unit *unit.UnitFile

	// New-style Jobs should have a populated UnitHash, and the contents of the Unit are stored separately in the Registry
	if !jm.UnitHash.Empty() {
		unit = r.getUnitByHash(jm.UnitHash)
		if unit == nil {
			log.Warningf("No Unit found in Registry for Job(%s)", jm.Name)
			return nil, nil
		}
	} else {
		// Old-style Jobs had "Payloads" instead of Units, also stored separately in the Registry
		unit, err = r.getUnitFromLegacyPayload(jm.Name)
		if err != nil {
			log.Errorf("Error retrieving legacy payload for Job(%s)", jm.Name)
			return nil, nil
		} else if unit == nil {
			log.Warningf("No Payload found in Registry for Job(%s)", jm.Name)
			return nil, nil
		}

		log.Infof("Migrating legacy Payload(%s)", jm.Name)
		if err := r.storeOrGetUnitFile(*unit); err != nil {
			log.Warningf("Unable to migrate legacy Payload: %v", err)
		}

		jm.UnitHash = unit.Hash()
		log.Infof("Updating Job(%s) with legacy payload Hash(%s)", jm.Name, jm.UnitHash)
		if err := r.updateJobObjectNode(&jm, node.ModifiedIndex); err != nil {
			log.Warningf("Unable to update Job(%s) with legacy payload Hash(%s): %v", jm.Name, jm.UnitHash, err)
		}
	}

	return job.NewJob(jm.Name, *unit), nil
}

// jobModel is used for serializing and deserializing Jobs stored in the Registry
type jobModel struct {
	Name     string
	UnitHash unit.Hash
}

// DestroyUnit removes a Job object from the repository, along with any legacy
// associated Payload and SignatureSet. It does not yet remove underlying
// Units from the repository.
func (r *EtcdRegistry) DestroyUnit(name string) error {
	req := etcd.Delete{
		Key:       path.Join(r.keyPrefix, jobPrefix, name),
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
	r.destroyLegacyPayload(name)
	r.destroySignatureSetOfJob(name)
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

// CreateUnit attempts to store a Unit and its associated unit file in the registry
func (r *EtcdRegistry) CreateUnit(u *job.Unit) (err error) {
	if err := r.storeOrGetUnitFile(u.Unit); err != nil {
		return err
	}

	jm := jobModel{
		Name:     u.Name,
		UnitHash: u.Unit.Hash(),
	}
	json, err := marshal(jm)
	if err != nil {
		return
	}

	req := etcd.Create{
		Key:   path.Join(r.keyPrefix, jobPrefix, u.Name, "object"),
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
	res, err := r.etcd.Do(&req)
	if err != nil {
		if isKeyNotFound(err) {
			err = nil
		}
		return job.JobStateInactive, err
	}

	return job.ParseJobState(res.Node.Value)
}

func (r *EtcdRegistry) SetJobTargetState(jobName string, state job.JobState) error {
	req := etcd.Set{
		Key:   r.jobTargetStatePath(jobName),
		Value: string(state),
	}
	_, err := r.etcd.Do(&req)
	return err
}

func (r *EtcdRegistry) ScheduleUnit(name string, machID string) error {
	req := etcd.Create{
		Key:   r.jobTargetAgentPath(name),
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
