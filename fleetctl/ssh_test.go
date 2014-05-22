package main

import (
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/resource"
	"github.com/coreos/fleet/unit"
)

func newFakeRegistryForSsh() registry.Registry {
	machines := []machine.MachineState{
		{"c31e44e1-f858-436e-933e-59c642517860", "1.2.3.4", map[string]string{"ping": "pong"}, "", resource.ResourceTuple{}},
		{"595989bb-cbb7-49ce-8726-722d6e157b4e", "5.6.7.8", map[string]string{"foo": "bar"}, "", resource.ResourceTuple{}},
		{"hello.service", "8.7.6.5", map[string]string{"foo": "bar"}, "", resource.ResourceTuple{}},
	}

	jobs := []job.Job{
		*job.NewJob("j1.service", unit.Unit{}),
		*job.NewJob("j2.service", unit.Unit{}),
		*job.NewJob("hello.service", unit.Unit{}),
	}

	states := map[string]*unit.UnitState{
		"j1.service":    unit.NewUnitState("loaded", "active", "listening", &machines[0]),
		"j2.service":    unit.NewUnitState("loaded", "inactive", "dead", &machines[1]),
		"hello.service": unit.NewUnitState("loaded", "inactive", "dead", &machines[2]),
	}

	reg := registry.NewFakeRegistry()
	reg.SetMachines(machines)
	reg.SetUnitStates(states)
	reg.SetJobs(jobs)

	return reg
}

func TestSshUnknownMachine(t *testing.T) {
	registryCtl = newFakeRegistryForSsh()

	_, ok := findAddressInMachineList("asdf")
	if ok {
		t.Error("Expected to not find any machine with the machine ID `asdf`")
	}
}

func TestSshFindMachine(t *testing.T) {
	registryCtl = newFakeRegistryForSsh()

	ip, _ := findAddressInMachineList("c31e44e1-f858-436e-933e-59c642517860")
	if ip != "1.2.3.4" {
		t.Errorf("Expected to return the host 1.2.3.4, but it was %s", ip)
	}
}

func TestSshFindMachineByUnknownJobName(t *testing.T) {
	registryCtl = newFakeRegistryForSsh()

	_, ok := findAddressInRunningUnits("asdf")
	if ok {
		t.Error("Expected to not find any machine with the job name `asdf`")
	}
}

func TestSshFindMachineByJobName(t *testing.T) {
	registryCtl = newFakeRegistryForSsh()

	ip, _ := findAddressInRunningUnits("j1")
	if ip != "1.2.3.4" {
		t.Errorf("Expected to return the host 1.2.3.4, but it was %s", ip)
	}
}

func TestGlobalLookupByUnknownArgument(t *testing.T) {
	registryCtl = newFakeRegistryForSsh()

	ip, err := globalMachineLookup([]string{"asdf"})
	if err != nil {
		t.Fatal("Expected to not find any error")
	}

	if ip != "" {
		t.Errorf("Expected to not find any host with the argument `asdf`")
	}
}

func TestGlobalLookupByMachineID(t *testing.T) {
	registryCtl = newFakeRegistryForSsh()

	ip, err := globalMachineLookup([]string{"c31e44e1-f858-436e-933e-59c642517860"})
	if err != nil {
		t.Fatal("Expected to not find any error")
	}

	if ip != "1.2.3.4" {
		t.Errorf("Expected to return the host 1.2.3.4, but it was %s", ip)
	}
}

func TestGlobalLookupByJobName(t *testing.T) {
	registryCtl = newFakeRegistryForSsh()

	ip, err := globalMachineLookup([]string{"j1"})
	if err != nil {
		t.Fatal("Expected to not find any error")
	}

	if ip != "1.2.3.4" {
		t.Errorf("Expected to return the host 1.2.3.4, but it was %s", ip)
	}
}

func TestGlobalLookupWithAmbiguousArgument(t *testing.T) {
	registryCtl = newFakeRegistryForSsh()

	_, err := globalMachineLookup([]string{"hello.service"})
	if err == nil {
		t.Fatal("Expected to find an error with an ambiguous argument")
	}
}
