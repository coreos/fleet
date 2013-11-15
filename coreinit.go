package main

import (
	"github.com/coreos/coreinit/agent"
	"github.com/coreos/coreinit/machine"
	"github.com/coreos/coreinit/registry"
	"github.com/coreos/coreinit/scheduler"
)

func main() {
	m := machine.New(machine.ReadLocalBootId())
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
