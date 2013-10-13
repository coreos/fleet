// Package registry is the primary object of coreos-registry
package registry

import (
	"path"
	"time"
	"net"
	"strings"

	"github.com/coreos/coreinit/machine"
	"github.com/coreos/go-etcd/etcd"
	"github.com/coreos/go-systemd/dbus"
)

const (
	DefaultServiceTTL = "10s"
	DefaultMachineTTL = "20m"
	keyPrefix = "/github.com/coreos/coreinit/"
	machinePrefix = "/machines/"
	systemPrefix = "/system/"
	refreshInterval = 2 // Refresh TTLs at 1/2 the TTL length
)

type Registry struct {
	Etcd *etcd.Client
	Systemd *dbus.Conn
	Machine *machine.Machine
	ServiceTTL string
}

// DoHeartbeat ensures that all of the units are registered at an interval of
// half of the TTL.
func (r *Registry) DoHeartbeat() {
	go r.doServiceHeartbeat()
	r.doMachineHeartbeat()
	return
}

func parseDuration(d string) time.Duration {
	duration, err := time.ParseDuration(d)
	if err != nil {
		panic(err)
	}

	return duration
}

func intervalFromTTL(ttl string) time.Duration {
	duration := parseDuration(ttl)
	return duration / refreshInterval
}

func (r *Registry) doServiceHeartbeat() {
	interval := intervalFromTTL(r.ServiceTTL)
	
	c := time.Tick(interval)
	for now := range c {
		println(now.String())
		r.SetAllUnits()
	}
}

func (r *Registry) doMachineHeartbeat() {
	interval := intervalFromTTL(DefaultMachineTTL)
	
	c := time.Tick(interval)
	for now := range c {
		println(now.String())
		r.SetAllUnits()
	}
}

func (r *Registry) SetMachine() {
	list := []string{}
	ifs, err := net.InterfaceAddrs()

	if err != nil {
		panic(err)
	}

	for _, k := range(ifs) {
		list = append(list, k.String())
	}
	
	key := path.Join(keyPrefix, machinePrefix, r.Machine.BootId, "network")
	d := parseDuration(DefaultMachineTTL)
	r.Etcd.Set(key, strings.Join(list, ","), uint64(d.Seconds()))
}

func (r *Registry) SetAllUnits() {
	units, err := r.Systemd.ListUnits()
	if err != nil {
		panic(err)
	}

	d := parseDuration(r.ServiceTTL)
	for _, u := range(units) {
		if u.ActiveState == "active" {
			println(u.Name, u.ActiveState)
			key := path.Join(keyPrefix, systemPrefix, u.Name, r.Machine.BootId)
			r.Etcd.Set(key, "active", uint64(d.Seconds()))
		}
	}
}

func NewRegistry(ttl string) (registry *Registry) {
	etcdC := etcd.NewClient(nil)
	mach := machine.NewMachine("")
	systemd := dbus.New()

	if ttl == "" {
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
