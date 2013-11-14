package registry

import (
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
func (r *Registry) GetAllMachines() map[string]machine.Machine {
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
		machines[machine.BootId] = *machine
	}

	return machines
}

func (r *Registry) SetMachineAddrs(machine *machine.Machine, addrs []machine.Addr, ttl time.Duration) {
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
		_, name := path.Split(kv.Key)
		payload := job.NewJobPayload(kv.Value)
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

func (r *Registry) ScheduleJob(job *job.Job, machine *machine.Machine) {
	key := path.Join(keyPrefix, machinePrefix, machine.BootId, schedulePrefix, job.Name)
	r.Etcd.Set(key, job.Payload.Value, 0)

	key = path.Join(keyPrefix, schedulePrefix, job.Name)
	r.Etcd.Delete(key)
}

// Persist the changes in a provided Machine's Job to Etcd with the provided TTL
func (r *Registry) UpdateMachineJob(machine *machine.Machine, job *job.Job, ttl uint64) {
	key := path.Join(keyPrefix, machinePrefix, machine.BootId, statePrefix, job.Name)
	r.Etcd.Set(key, job.State.State, ttl)
}

// Attempt to acquire a lock in Etcd on an arbitrary string. Returns true if
// successful, otherwise false.
func (r *Registry) AcquireLock(name string, context string, ttl time.Duration) bool {
	key := path.Join(keyPrefix, lockPrefix, name)
	_, err := r.Etcd.Create(key, context, uint64(ttl.Seconds()))
	return err == nil
}
