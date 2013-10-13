// Package registry is the primary object of coreos-registry
package registry

import (
	"path"
	"time"

	"github.com/coreos/muffins/machine"
	"github.com/coreos/go-etcd/etcd"
	"github.com/coreos/go-systemd/dbus"
)

const keyPrefix = "/core-registry/system/"
// TODO: Make this a variable
const DefaultServiceTTL = 10

type Registry struct {
	Etcd *etcd.Client
	Systemd *dbus.Conn
	Machine *machine.Machine
	ServiceTTL uint64
}

// heartbeat ensures that all of the units 
func (i *Registry) StartHeartbeat() {
	// TODO: Use the new directory TTL in the v2 API instead of
	// heartbeating all of the units
	interval := i.ServiceTTL / 2.0
	
	c := time.Tick(time.Duration(interval) * time.Second)
	for now := range c {
		println(now.String())
		go i.SetAllUnits()
	}
}

func (i *Registry) SetAllUnits() {
	units, err := i.Systemd.ListUnits()
	if err != nil {
		panic(err)
	}

	for _, u := range(units) {
		if u.ActiveState == "active" {
			println(u.Name, u.ActiveState)
			key := path.Join(keyPrefix, u.Name, i.Machine.BootId)
			i.Etcd.Set(key, "active", i.ServiceTTL)
		}
	}
}

func NewRegistry(ttl uint64) (registry *Registry) {
	etcdC := etcd.NewClient(nil)
	mach := machine.NewMachine("")
	systemd := dbus.New()

	if ttl == 0 {
		ttl = DefaultServiceTTL
	}

	registry = &Registry{etcdC, systemd, mach, ttl}

	return registry
}

/*
func startUnit() {
	script := []string{"/bin/sh", "-c",
		"while true; do echo goodbye world; sleep 1; done"}

	jobid, err := i.Systemd.StartTransientUnit("hello.service",
		"replace",
		dbus.PropExecStart(script, false))
	fmt.Println(jobid, err)
}
*/


