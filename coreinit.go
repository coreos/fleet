package main

import (
	"github.com/coreos/muffins/registry"
)

func main() {
	r := registry.NewRegistry(10)
	r.SetAllUnits()
	r.StartHeartbeat()
}
