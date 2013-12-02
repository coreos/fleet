package main

import (
	"flag"
	"os"

	"github.com/coreos/coreinit/agent"
	"github.com/coreos/coreinit/engine"
	"github.com/coreos/coreinit/machine"
	"github.com/coreos/coreinit/registry"
)

func main() {
	var bootId string

	f := flag.NewFlagSet(os.Args[0], 1)
	f.StringVar(&bootId, "bootid", "", "Provide a user-generated boot ID. This will override the actual boot ID of the machine.")
	f.Parse(os.Args[1:])

	if bootId == "" {
		bootId = machine.ReadLocalBootId()
	}

	m := machine.New(bootId)
	r := registry.New()
	es := registry.NewEventStream()

	a := agent.New(r, es, m, "")
	go a.Run()

	e := engine.New(r, es, m)
	e.Run()
}
