package main

import (
	"github.com/coreos/coreinit/agent"
	"github.com/coreos/coreinit/registry"
)

func main() {
	r := registry.NewRegistry()
	a := agent.NewAgent(r, "")

	// Push the initial state to the registry
	a.UpdateUnits()
	a.UpdateMachine()

	// Kick off the heartbeating process
	a.DoHeartbeat()
}
