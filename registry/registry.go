// Package registry is the primary object of coreos-registry
package registry

import (
	"encoding/json"
	"path"
	"strings"
	"time"
	"net"

	"github.com/coreos/go-etcd/etcd"
	systemdDbus "github.com/coreos/go-systemd/dbus"
	"github.com/guelfey/go.dbus"
)

const (
	DefaultServiceTTL = "10s"
	DefaultMachineTTL = "20m"
	keyPrefix = "/coreos.com/coreinit/"
	machinePrefix = "/machines/"
	systemPrefix = "/system/"
	refreshInterval = 2 // Refresh TTLs at 1/2 the TTL length
)

type Registry struct {
	Etcd *etcd.Client
	Systemd *systemdDbus.Conn
	Machine *Machine
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
		r.SetMachine()
	}
}

func (r *Registry) SetMachine() {
	var addrs []Addr
	ifs, err := net.Interfaces()

	if err != nil {
		panic(err)
	}

	shouldAppend := func(i net.Interface) bool {
		if (i.Flags & net.FlagLoopback) == net.FlagLoopback {
			return false
		}

		if (i.Flags & net.FlagUp) != net.FlagUp {
			return false
		}

		return true
	}

	for _, k := range(ifs) {
		if shouldAppend(k) != true {
			continue
		}
		kaddrs, _ := k.Addrs()
		for _, j := range(kaddrs) {
			if strings.HasPrefix(j.String(), "fe80::") == true {
				continue
			}
			addrs = append(addrs, Addr{j.String(), j.Network()})
		}
	}

	addrsjson, err := json.Marshal(addrs)
	if err != nil {
		panic(err)
	}

	key := path.Join(keyPrefix, machinePrefix, r.Machine.BootId, "addrs")
	d := parseDuration(DefaultMachineTTL)

	r.Etcd.Set(key, string(addrsjson), uint64(d.Seconds()))
}

func unitPath(unit string) dbus.ObjectPath {
	prefix := "/org/freedesktop/systemd1/unit/"
	split := strings.Split(unit, ".")
	unit = strings.Join(split, "_2e")
	unitPath := path.Join(prefix, unit)
	return dbus.ObjectPath(unitPath)
}

func (r *Registry) SetAllUnits() {
	object := unitPath("local.target")
	info, err := r.Systemd.GetUnitInfo(object)
	if err != nil {
		panic(err)
	}

	localUnits := info["Wants"].Value().([]string)

	d := parseDuration(r.ServiceTTL)
	for _, u := range(localUnits) {
		info, err := r.Systemd.GetUnitInfo(unitPath(u))
		if err != nil {
			panic(err)
		}

		state := info["ActiveState"].Value().(string)

		if state == "active" {
			println(u, state)
			key := path.Join(keyPrefix, systemPrefix, u, r.Machine.BootId)
			r.Etcd.Set(key, "active", uint64(d.Seconds()))
		}
	}
}

func NewRegistry(ttl string) (registry *Registry) {
	etcdC := etcd.NewClient(nil)
	mach := NewMachine("")
	systemd := systemdDbus.New()

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
