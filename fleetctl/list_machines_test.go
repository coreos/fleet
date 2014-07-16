package main

import (
	"testing"

	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/resource"
)

func newTestRegistryForListMachines() registry.Registry {
	m := []machine.MachineState{
		machine.MachineState{ID: "mnopqr"},
		machine.MachineState{ID: "abcdef"},
		machine.MachineState{ID: "ghijkl"},
	}

	reg := registry.NewFakeRegistry()
	reg.SetMachines(m)

	return reg
}

func TestListMachinesFieldsToStrings(t *testing.T) {
	id := "4d389537d9d14bdabe8be54a9c29f68d"
	ip := "192.0.2.1"
	metadata := map[string]string{
		"foo":  "bar",
		"ping": "pong",
	}
	ver := "v9.9.9"
	res := resource.ResourceTuple{10, 1024, 1024}

	ms := &machine.MachineState{
		ID:             id,
		PublicIP:       ip,
		Metadata:       metadata,
		Version:        ver,
		TotalResources: res,
	}

	val := listMachinesFields["machine"](ms, false)
	assertEqual(t, "machine", "4d389537...", val)

	val = listMachinesFields["machine"](ms, true)
	assertEqual(t, "machine", "4d389537d9d14bdabe8be54a9c29f68d", val)

	val = listMachinesFields["ip"](ms, false)
	assertEqual(t, "ip", "192.0.2.1", val)

	val = listMachinesFields["metadata"](ms, false)
	assertEqual(t, "metadata", "foo=bar,ping=pong", val)
}

func TestListMachinesFieldsEmpty(t *testing.T) {
	id := "4d389537d9d14bdabe8be54a9c29f68d"
	ip := ""
	metadata := map[string]string{}
	ver := "v9.9.9"
	res := resource.ResourceTuple{10, 1024, 1024}

	ms := &machine.MachineState{
		ID:             id,
		PublicIP:       ip,
		Metadata:       metadata,
		Version:        ver,
		TotalResources: res,
	}

	for _, tt := range []string{"ip", "metadata"} {
		f := listMachinesFields[tt](ms, false)
		assertEqual(t, tt, "-", f)
	}
}
