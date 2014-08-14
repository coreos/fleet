package main

import (
	"time"

	"github.com/coreos/fleet/client"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/unit"
)

type BlockedFakeRegistry struct {
	EchoAttempts int
	registry.FakeRegistry
}

func (b BlockedFakeRegistry) Unit(name string) (*job.Unit, error) {
	if name == "hello.service" {
		time.Sleep(500 * time.Millisecond)
	}

	if name == "echo.service" {
		if b.EchoAttempts != 0 {
			b.EchoAttempts--
			return nil, nil
		}
	}

	return b.FakeRegistry.Unit(name)
}

func setupRegistryForStart(echoAttempts int) {
	m1 := machine.MachineState{
		ID:       "c31e44e1-f858-436e-933e-59c642517860",
		PublicIP: "1.2.3.4",
		Metadata: map[string]string{"ping": "pong"},
	}
	m2 := machine.MachineState{
		ID:       "595989bb-cbb7-49ce-8726-722d6e157b4e",
		PublicIP: "5.6.7.8",
		Metadata: map[string]string{"foo": "bar"},
	}
	m3 := machine.MachineState{
		ID:       "520983A8-FB9C-4A68-B49C-CED5BB2E9D08",
		Metadata: map[string]string{"foo": "bar"},
	}

	js := unit.NewUnitState("loaded", "active", "listening", m1.ID)
	js2 := unit.NewUnitState("loaded", "inactive", "dead", m2.ID)
	js3 := unit.NewUnitState("loaded", "inactive", "dead", m2.ID)
	js4 := unit.NewUnitState("loaded", "inactive", "dead", m3.ID)

	states := map[string]*unit.UnitState{"pong.service": js, "hello.service": js2, "echo.service": js3, "private.service": js4}
	machines := []machine.MachineState{m1, m2, m3}

	reg := registry.NewFakeRegistry()
	reg.SetMachines(machines)
	reg.SetUnitStates(states)

	cAPI = &client.RegistryClient{&BlockedFakeRegistry{echoAttempts, *reg}}
}
