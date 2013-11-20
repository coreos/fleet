package registry

import (
	"bytes"
	"encoding/json"
	"path"
	"time"

	"github.com/coreos/coreinit/job"
	"github.com/coreos/coreinit/machine"
	"github.com/coreos/go-etcd/etcd"
)

const (
	keyPrefix      = "/coreos.com/coreinit/"
	lockPrefix     = "/locks/"
	machinePrefix  = "/machines/"
	schedulePrefix = "/schedule/"
	statePrefix    = "/state/"
)

type Registry struct {
	Etcd *etcd.Client
}

func New() (registry *Registry) {
	etcdC := etcd.NewClient(nil)
	registry = &Registry{etcdC}
	return registry
}

// Describe the list of all known Machines
func (r *Registry) GetActiveMachines() map[string]machine.Machine {
	key := path.Join(keyPrefix, machinePrefix)
	resp, err := r.Etcd.Get(key, false)

	// Assume the error was KeyNotFound and return an empty data structure
	if err != nil {
		return make(map[string]machine.Machine, 0)
	}

	machines := make(map[string]machine.Machine, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		_, bootId := path.Split(kv.Key)
		machine := machine.New(bootId)

		// This is a hacky way of telling if a Machine is reporting state
		addrs := r.GetMachineAddrs(machine)
		if len(addrs) > 0 {
			machines[machine.BootId] = *machine
		}
	}

	return machines
}

func (r *Registry) GetMachineAddrs(m *machine.Machine) []machine.IPAddress {
	key := path.Join(keyPrefix, machinePrefix, m.BootId, "addrs")
	resp, err :=r.Etcd.Get(key, false)

	addrs := make([]machine.IPAddress, 0)

	// Assume this is KeyNotFound and return an empty data structure
	if err != nil {
		return addrs
	}

	json.Unmarshal([]byte(resp.Value), &addrs)

	return addrs
}

func (r *Registry) SetMachineAddrs(machine *machine.Machine, addrs []machine.IPAddress, ttl time.Duration) {
	addrsjson, err := json.Marshal(addrs)
	if err != nil {
		panic(err)
	}

	key := path.Join(keyPrefix, machinePrefix, machine.BootId, "addrs")
	r.Etcd.Set(key, string(addrsjson), uint64(ttl.Seconds()))
}

// Private helper method that takes a path to an Etcd directory and returns
// all of the items as job.Job objects.
func (r *Registry) getJobsAtPath(key string) map[string]job.Job {
	resp, err := r.Etcd.Get(key, false)

	// Assume the error was KeyNotFound and return an empty data structure
	if err != nil {
		return make(map[string]job.Job, 0)
	}

	jobs := make(map[string]job.Job, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		name := path.Base(kv.Key)
		nameBytes := []byte(name)

		var payloadType string
		if bytes.HasSuffix(nameBytes, []byte(".service")) {
			payloadType = "systemd-service"
		} else if bytes.HasSuffix(nameBytes, []byte(".socket")) {
			payloadType = "systemd-socket"
		} else {
			// Unable to handle this job type
			continue
		}

		payload := job.NewJobPayload(payloadType, kv.Value)
		job := job.NewJob(name, nil, payload)
		jobs[job.Name] = *job
	}

	return jobs
}

// Describe the list of jobs that have not yet been scheduled to a Machine
func (r *Registry) GetGlobalJobs() map[string]job.Job {
	key := path.Join(keyPrefix, schedulePrefix)
	return r.getJobsAtPath(key)
}

// Describe the list of jobs a given Machine is scheduled to run
func (r *Registry) GetMachineJobs(machine *machine.Machine) map[string]job.Job {
	key := path.Join(keyPrefix, machinePrefix, machine.BootId, schedulePrefix)
	return r.getJobsAtPath(key)
}

func (r *Registry) GetJobState(j *job.Job) *job.JobState {
	key := path.Join(keyPrefix, statePrefix, j.Name)
	resp, err := r.Etcd.Get(key, false)

	// Assume the error was KeyNotFound and return an empty data structure
	if err != nil {
		return nil
	}

	var state job.JobState
	json.Unmarshal([]byte(resp.Value), &state)
	return &state
}

func (r *Registry) ScheduleJob(job *job.Job, machine *machine.Machine) {
	key := path.Join(keyPrefix, machinePrefix, machine.BootId, schedulePrefix, job.Name)
	r.Etcd.Set(key, job.Payload.Value, 0)
}

// StartJob adds a job to the schedule to be started.
func (r *Registry) StartJob(job *job.Job) {
	key := path.Join(keyPrefix, schedulePrefix, job.Name)
	r.Etcd.Set(key, job.Payload.Value, 0)
}

// StopJob removes the job from the global and machine schedule.
func (r *Registry) StopJob(job *job.Job) {
	key := path.Join(keyPrefix, schedulePrefix, job.Name)
	r.Etcd.Delete(key)

	state := r.GetJobState(job)

	if state != nil {
		key := path.Join(keyPrefix, machinePrefix, state.Machine.BootId, schedulePrefix, job.Name)
		r.Etcd.Delete(key)
	}
}

// Persist the changes in a provided Machine's Job to Etcd with the provided TTL
func (r *Registry) UpdateJob(job *job.Job, ttl uint64) {
	key := path.Join(keyPrefix, statePrefix, job.Name)
	encoded, err := json.Marshal(job.State)
	if err != nil {
		panic(err)
	}
	r.Etcd.Set(key, string(encoded), ttl)
}

// Attempt to acquire a lock in Etcd on an arbitrary string. Returns true if
// successful, otherwise false.
func (r *Registry) AcquireLock(name string, context string, ttl time.Duration) bool {
	key := path.Join(keyPrefix, lockPrefix, name)
	_, err := r.Etcd.Create(key, context, uint64(ttl.Seconds()))
	return err == nil
}
