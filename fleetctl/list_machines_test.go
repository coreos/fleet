package main

import (
	"testing"

	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
)

func newTestRegistryForListMachines() registry.Registry {
	m := []machine.MachineState{
		machine.MachineState{ID: "mnopqr"},
		machine.MachineState{ID: "abcdef"},
		machine.MachineState{ID: "ghijkl"},
	}
	return registry.NewTestRegistry(m, nil, nil, nil, nil)
}

func TestGetAllMachines(t *testing.T) {
	registryCtl = newTestRegistryForListMachines()
	machines, sortable, err := findAllMachines()
	if err != nil {
		t.Fatalf("Unexpected error getting all machines: %v\n", err)
	}
	if len(machines) != 3 {
		t.Fatalf("Expected to find three machines, got: %v\n", machines)
	}

	if sortable[0] != "abcdef" {
		t.Errorf("Expected to find abcdef as the first machine, but it was %s\n", sortable[0])
	}

	if sortable[2] != "mnopqr" {
		t.Errorf("Expected to find mnopqr as the last machine, but it was %s\n", sortable[2])
	}
}
