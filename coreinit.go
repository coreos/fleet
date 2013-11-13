package main

import (
	"github.com/coreos/coreinit/agent"
	"github.com/coreos/coreinit/registry"
)

func main() {
	r := registry.New()
	a := agent.New(r, "")

	// Push the initial state to the registry
	a.UpdateUnits()
	a.UpdateMachine()

	// Kick off the heartbeating process
	a.DoHeartbeat()
}
