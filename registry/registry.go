package registry

import (
	"encoding/json"
	"log"
	"path"
	"time"

	"github.com/coreos/coreinit/machine"
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

func (r *Registry) SetMachineAddrs(machine *machine.Machine, addrs []machine.Addr, ttl time.Duration) {
	addrsjson, err := json.Marshal(addrs)
	if err != nil {
		panic(err)
	}

	key := path.Join(keyPrefix, machinePrefix, machine.BootId, "addrs")
	r.Etcd.Set(key, string(addrsjson), uint64(ttl.Seconds()))
}

func (r *Registry) GetUnits() map[string]string {
	key := path.Join(keyPrefix, schedulePrefix)
	objects, _ := r.Etcd.Get(key)
	units := make(map[string]string, len(objects))
	for _, obj := range objects {
		_, unitName := path.Split(obj.Key)
		units[unitName] = obj.Value
	}
	return units
}

func (r *Registry) SetUnitState(machine *machine.Machine, unit string, state string, ttl uint64) {
	key := path.Join(keyPrefix, systemPrefix, unit, machine.BootId)
	log.Println("Setting unit state")
	r.Etcd.Set(key, state, ttl)
}

func (r *Registry) DeleteUnit(unit string) {
	key := path.Join(keyPrefix, schedulePrefix, unit)
	r.Etcd.Delete(key)
}
