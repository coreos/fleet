package main

import (
	"os"
	"flag"

	"github.com/coreos/coreinit/agent"
	"github.com/coreos/coreinit/machine"
	"github.com/coreos/coreinit/registry"
	"github.com/coreos/coreinit/scheduler"
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
	a := agent.New(r, m, "")

	// Push the initial state to the registry
	a.UpdateJobs()
	a.UpdateMachine()

	// Kick off the heartbeating process
	go a.DoHeartbeat()

	s := scheduler.New(r, m)
	s.DoSchedule()
}
