package main

import (
	"flag"

	"github.com/coreos/go-etcd/etcd"
	"github.com/golang/glog"

	"github.com/coreos/coreinit/agent"
	"github.com/coreos/coreinit/engine"
	"github.com/coreos/coreinit/machine"
	"github.com/coreos/coreinit/registry"
)

func main() {
	var bootId string

	flag.StringVar(&bootId, "bootid", "", "Provide a user-generated boot ID. This will override the actual boot ID of the machine.")
	flag.Parse()

	if bootId == "" {
		bootId = machine.ReadLocalBootId()
	}

	if glog.V(2) {
		etcd.OpenDebug()
	}

	m := machine.New(bootId)
	r := registry.New()
	es := registry.NewEventStream()

	a := agent.New(r, es, m, "")
	go a.Run()

	e := engine.New(r, es, m)
	e.Run()
}
