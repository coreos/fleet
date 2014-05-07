package main

import (
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/unit"
)

func newTestRegistryForSsh() registry.Registry {
	machines := []machine.MachineState{
		machine.MachineState{"c31e44e1-f858-436e-933e-59c642517860", "1.2.3.4", map[string]string{"ping": "pong"}, ""},
		machine.MachineState{"595989bb-cbb7-49ce-8726-722d6e157b4e", "5.6.7.8", map[string]string{"foo": "bar"}, ""},
		machine.MachineState{"hello.service", "8.7.6.5", map[string]string{"foo": "bar"}, ""},
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

	return TestRegistry{machines: machines, jobStates: states, jobs: jobs}
}

func TestSshUnknownMachine(t *testing.T) {
	registryCtl = newTestRegistryForSsh()

	_, ok := findAddressInMachineList("asdf")
	if ok {
		t.Error("Expected to not find any machine with the machine ID `asdf`")
	}
}

func TestSshFindMachine(t *testing.T) {
	registryCtl = newTestRegistryForSsh()

	ip, _ := findAddressInMachineList("c31e44e1-f858-436e-933e-59c642517860")
	if ip != "1.2.3.4:22" {
		t.Errorf("Expected to return the host 1.2.3.4:22, but it was %s", ip)
	}
}

func TestSshFindMachineByUnknownJobName(t *testing.T) {
	registryCtl = newTestRegistryForSsh()

	_, ok := findAddressInRunningUnits("asdf")
	if ok {
		t.Error("Expected to not find any machine with the job name `asdf`")
	}
}

func TestSshFindMachineByJobName(t *testing.T) {
	registryCtl = newTestRegistryForSsh()

	ip, _ := findAddressInRunningUnits("j1")
	if ip != "1.2.3.4:22" {
		t.Errorf("Expected to return the host 1.2.3.4:22, but it was %s", ip)
	}
}

func TestGlobalLookupByUnknownArgument(t *testing.T) {
	registryCtl = newTestRegistryForSsh()

	ip, err := globalMachineLookup([]string{"asdf"})
	if err != nil {
		t.Fatal("Expected to not find any error")
	}

	if ip != "" {
		t.Errorf("Expected to not find any host with the argument `asdf`")
	}
}

func TestGlobalLookupByMachineID(t *testing.T) {
	registryCtl = newTestRegistryForSsh()

	ip, err := globalMachineLookup([]string{"c31e44e1-f858-436e-933e-59c642517860"})
	if err != nil {
		t.Fatal("Expected to not find any error")
	}

	if ip != "1.2.3.4:22" {
		t.Errorf("Expected to return the host 1.2.3.4:22, but it was %s", ip)
	}
}

func TestGlobalLookupByJobName(t *testing.T) {
	registryCtl = newTestRegistryForSsh()

	ip, err := globalMachineLookup([]string{"j1"})
	if err != nil {
		t.Fatal("Expected to not find any error")
	}

	if ip != "1.2.3.4:22" {
		t.Errorf("Expected to return the host 1.2.3.4:22, but it was %s", ip)
	}
}

func TestGlobalLookupWithAmbiguousArgument(t *testing.T) {
	registryCtl = newTestRegistryForSsh()

	_, err := globalMachineLookup([]string{"hello.service"})
	if err == nil {
		t.Fatal("Expected to find an error with an ambiguous argument")
	}
}
