// Package registry is the primary object of coreos-registry
package registry

import (
	"encoding/json"
	"path"

	"github.com/coreos/go-etcd/etcd"
)

const (
	keyPrefix = "/coreos.com/coreinit/"
	machinePrefix = "/machines/"
	systemPrefix = "/system/"
	schedulePrefix = "/schedule/"
)

type Registry struct {
	Etcd *etcd.Client
}

func NewRegistry() (registry *Registry) {
	etcdC := etcd.NewClient(nil)
	registry = &Registry{etcdC}
	return registry
}

func (r *Registry) SetMachineAddrs(machine *Machine, addrs []Addr) {
	addrsjson, err := json.Marshal(addrs)
	if err != nil {
		panic(err)
	}

	key := path.Join(keyPrefix, machinePrefix, machine.BootId, "addrs")
	d := parseDuration(DefaultMachineTTL)

	r.Etcd.Set(key, string(addrsjson), uint64(d.Seconds()))
}

func (r *Registry) SetUnitState(machine *Machine, unit string, state string, ttl uint64) {
	key := path.Join(keyPrefix, systemPrefix, unit, machine.BootId)
	println("Setting unit state")
	r.Etcd.Set(key, state, ttl)
}
