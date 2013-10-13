package main

import (
	"github.com/coreos/coreinit/registry"
)

func main() {
	r := registry.NewRegistry("")

	// Push the initial state to the registry
	r.SetAllUnits()
	r.SetMachine()

	// Kick off the heartbeating process
	r.DoHeartbeat()
}
