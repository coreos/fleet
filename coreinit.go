package main

import (
	"flag"
	"os"

	"github.com/coreos/go-etcd/etcd"

	"github.com/coreos/coreinit/agent"
	"github.com/coreos/coreinit/engine"
	"github.com/coreos/coreinit/machine"
	"github.com/coreos/coreinit/registry"
)

func main() {
	var bootId string
	var debug bool

	f := flag.NewFlagSet(os.Args[0], 1)
	f.StringVar(&bootId, "bootid", "", "Provide a user-generated boot ID. This will override the actual boot ID of the machine.")
	f.BoolVar(&debug, "debug", false, "Generate debug-level output in server logs.")
	f.Parse(os.Args[1:])

	if bootId == "" {
		bootId = machine.ReadLocalBootId()
	}

	if debug {
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
