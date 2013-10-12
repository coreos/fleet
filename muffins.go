package main

import (
	"path"

	"github.com/coreos/muffins/machine"
	"github.com/coreos/go-etcd/etcd"
	"github.com/coreos/go-systemd/dbus"
)

const keyPrefix = "/core-init/system/"

func init() {
}

/*
func startUnit() {
	script := []string{"/bin/sh", "-c",
		"while true; do echo goodbye world; sleep 1; done"}

	jobid, err := systemd.StartTransientUnit("hello.service",
		"replace",
		dbus.PropExecStart(script, false))
	fmt.Println(jobid, err)
}
*/

func main() {
	etcdC := etcd.NewClient(nil)
	mach := machine.NewMachine("")
	prefix := path.Join(keyPrefix, mach.BootId)

	systemd := dbus.New()
	units, err := systemd.ListUnits()
	if err != nil {
		panic(err)
	}

	for _, u := range(units) {
		if u.ActiveState == "active" {
			println(u.Name, u.ActiveState)
			key := path.Join(prefix, u.Name)
			etcdC.Set(key, "active", 0)
		}
	}
}
