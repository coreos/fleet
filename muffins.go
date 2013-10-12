package main

import (
	"path"
	"time"

	"github.com/coreos/muffins/machine"
	"github.com/coreos/go-etcd/etcd"
	"github.com/coreos/go-systemd/dbus"
)

const keyPrefix = "/core-init/system/"
// TODO: Make this a variable
const ttl = 10

type Init struct {
	Etcd *etcd.Client
	Systemd *dbus.Conn
	Machine *machine.Machine
}

func init() {
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

// heartbeat ensures that all of the units 
func (i *Init) StartHeartbeat() {
	// TODO: Use the new directory TTL in the v2 API instead of
	// heartbeating all of the units
	interval := ttl / 2.0
	
	c := time.Tick(time.Duration(interval) * time.Second)
	for now := range c {
		println(now.String())
		go i.SetAllUnits()
	}
}

func (i *Init) SetAllUnits() {
	units, err := i.Systemd.ListUnits()
	if err != nil {
		panic(err)
	}

	for _, u := range(units) {
		if u.ActiveState == "active" {
			println(u.Name, u.ActiveState)
			key := path.Join(keyPrefix, u.Name, i.Machine.BootId)
			i.Etcd.Set(key, "active", 0)
		}
	}
}

func main() {
	etcdC := etcd.NewClient(nil)
	mach := machine.NewMachine("")
	systemd := dbus.New()

	init := Init{etcdC, systemd, mach}

	init.SetAllUnits()
	init.StartHeartbeat()
}
