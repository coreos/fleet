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
	machinePrefix  = "/machines/"
	schedulePrefix = "/schedule/"
	statePrefix     = "/state/"
)

type Registry struct {
	Etcd *etcd.Client
}

func New() (registry *Registry) {
	etcdC := etcd.NewClient(nil)
	registry = &Registry{etcdC}
	return registry
}

func (r *Registry) SetMachineAddrs(machine *machine.Machine, addrs []machine.Addr, ttl time.Duration) {
	addrsjson, err := json.Marshal(addrs)
	if err != nil {
		panic(err)
	}

	key := path.Join(keyPrefix, machinePrefix, machine.BootId, "addrs")
	r.Etcd.Set(key, string(addrsjson), uint64(ttl.Seconds()))
}

// Descibe the list of jobs a given Machine is scheduled to run
func (r *Registry) GetMachineJobs(machine *machine.Machine) map[string]job.Job {
	key := path.Join(keyPrefix, machinePrefix, machine.BootId, schedulePrefix)
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

// Persist the changes in a provided Machine's Job to Etcd with the provided TTL
func (r *Registry) UpdateMachineJob(machine *machine.Machine, job *job.Job, ttl uint64) {
	key := path.Join(keyPrefix, machinePrefix, machine.BootId, statePrefix, job.Name)
	r.Etcd.Set(key, job.State.State, ttl)
}
