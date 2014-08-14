package main

import (
	"testing"

	"github.com/coreos/fleet/client"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/unit"
)

func newMachineState(id, ip string, md map[string]string) machine.MachineState {
	return machine.MachineState{
		ID:       id,
		PublicIP: ip,
		Metadata: md,
	}
}

func newFakeRegistryForSsh() client.API {
	// clear machineStates for every invocation
	machineStates = nil
	machines := []machine.MachineState{
		newMachineState("c31e44e1-f858-436e-933e-59c642517860", "1.2.3.4", map[string]string{"ping": "pong"}),
		newMachineState("595989bb-cbb7-49ce-8726-722d6e157b4e", "5.6.7.8", map[string]string{"foo": "bar"}),
		newMachineState("hello.service", "8.7.6.5", map[string]string{"foo": "bar"}),
	}

	jobs := []job.Job{
		job.Job{Name: "j1.service", Unit: unit.UnitFile{}, TargetMachineID: machines[0].ID},
		job.Job{Name: "j2.service", Unit: unit.UnitFile{}, TargetMachineID: machines[1].ID},
		job.Job{Name: "hello.service", Unit: unit.UnitFile{}, TargetMachineID: machines[2].ID},
	}

	states := map[string]*unit.UnitState{
		"j1.service":    unit.NewUnitState("loaded", "active", "listening", machines[0].ID),
		"j2.service":    unit.NewUnitState("loaded", "inactive", "dead", machines[1].ID),
		"hello.service": unit.NewUnitState("loaded", "inactive", "dead", machines[2].ID),
	}

	reg := registry.NewFakeRegistry()
	reg.SetMachines(machines)
	reg.SetUnitStates(states)
	reg.SetJobs(jobs)

	return &client.RegistryClient{reg}
}

func TestSshUnknownMachine(t *testing.T) {
	cAPI = newFakeRegistryForSsh()

	_, ok := findAddressInMachineList("asdf")
	if ok {
		t.Error("Expected to not find any machine with the machine ID `asdf`")
	}
}

func TestSshFindMachine(t *testing.T) {
	cAPI = newFakeRegistryForSsh()

	ip, _ := findAddressInMachineList("c31e44e1-f858-436e-933e-59c642517860")
	if ip != "1.2.3.4" {
		t.Errorf("Expected to return the host 1.2.3.4, but it was %s", ip)
	}
}

func TestSshFindMachineByUnknownUnitName(t *testing.T) {
	cAPI = newFakeRegistryForSsh()

	_, ok := findAddressInRunningUnits("asdf")
	if ok {
		t.Error("Expected to not find any machine with the unit name `asdf`")
	}
}

func TestSshFindMachineByUnitName(t *testing.T) {
	cAPI = newFakeRegistryForSsh()

	ip, _ := findAddressInRunningUnits("j1")
	if ip != "1.2.3.4" {
		t.Errorf("Expected to return the host 1.2.3.4, but it was %s", ip)
	}
}

func TestGlobalLookupByUnknownArgument(t *testing.T) {
	cAPI = newFakeRegistryForSsh()

	ip, err := globalMachineLookup([]string{"asdf"})
	if err != nil {
		t.Fatal("Expected to not find any error")
	}

	if ip != "" {
		t.Errorf("Expected to not find any host with the argument `asdf`")
	}
}

func TestGlobalLookupByMachineID(t *testing.T) {
	cAPI = newFakeRegistryForSsh()

	ip, err := globalMachineLookup([]string{"c31e44e1-f858-436e-933e-59c642517860"})
	if err != nil {
		t.Fatal("Expected to not find any error")
	}

	if ip != "1.2.3.4" {
		t.Errorf("Expected to return the host 1.2.3.4, but it was %s", ip)
	}
}

func TestGlobalLookupByUnitName(t *testing.T) {
	cAPI = newFakeRegistryForSsh()

	ip, err := globalMachineLookup([]string{"j1"})
	if err != nil {
		t.Fatal("Expected to not find any error")
	}

	if ip != "1.2.3.4" {
		t.Errorf("Expected to return the host 1.2.3.4, but it was %s", ip)
	}
}

func TestGlobalLookupWithAmbiguousArgument(t *testing.T) {
	cAPI = newFakeRegistryForSsh()

	_, err := globalMachineLookup([]string{"hello.service"})
	if err == nil {
		t.Fatal("Expected to find an error with an ambiguous argument")
	}
}
