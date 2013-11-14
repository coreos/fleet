package registry

import (
	"encoding/json"
	"path"
	"time"

	"github.com/coreos/coreinit/machine"
	"github.com/coreos/go-etcd/etcd"
)

const (
	keyPrefix = "/coreos.com/coreinit/"
	machinePrefix = "/machines/"
	schedulePrefix = "/schedule/"
	unitPrefix = "/units/"
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

func (r *Registry) GetScheduledUnits(machine *machine.Machine) map[string]string {
	key := path.Join(keyPrefix, machinePrefix, machine.BootId, schedulePrefix)
	resp, err := r.Etcd.Get(key, false)

	// Assume the error was KeyNotFound and return an empty data structure
	if err != nil {
		return make(map[string]string, 0)
	}

	units := make(map[string]string, len(resp.Kvs))
	for _, obj := range resp.Kvs {
		_, unitName := path.Split(obj.Key)
		units[unitName] = obj.Value
	}

	return units
}

func (r *Registry) SetUnitState(machine *machine.Machine, unit string, state string, ttl uint64) {
	key := path.Join(keyPrefix, machinePrefix, machine.BootId, unitPrefix, unit)
	r.Etcd.Set(key, state, ttl)
}
