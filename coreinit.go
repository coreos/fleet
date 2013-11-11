package main

import (
	"github.com/coreos/coreinit/registry"
)

func main() {
	r := registry.NewRegistry()
	a := registry.NewAgent(r, "")

	// Push the initial state to the registry
	a.SetAllUnits()
	a.SetMachine()

	// Kick off the heartbeating process
	a.DoHeartbeat()
}
