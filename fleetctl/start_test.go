package main

import (
	"time"

	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/resource"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/unit"
)

type BlockedTestRegistry struct {
	EchoAttempts int
	registry.TestRegistry
}

func (b BlockedTestRegistry) GetJobTarget(name string) (string, error) {
	if name == "hello.service" {
		time.Sleep(500 * time.Millisecond)
	}

	if name == "echo.service" {
		if b.EchoAttempts != 0 {
			b.EchoAttempts--
			return "", nil
		}
	}

	return b.TestRegistry.GetJobTarget(name)
}

func setupRegistryForStart(echoAttempts int) {
	m1 := machine.MachineState{"c31e44e1-f858-436e-933e-59c642517860", "1.2.3.4", map[string]string{"ping": "pong"}, "", resource.ResourceTuple{}}
	m2 := machine.MachineState{"595989bb-cbb7-49ce-8726-722d6e157b4e", "5.6.7.8", map[string]string{"foo": "bar"}, "", resource.ResourceTuple{}}
	m3 := machine.MachineState{"520983A8-FB9C-4A68-B49C-CED5BB2E9D08", "", map[string]string{"foo": "bar"}, "", resource.ResourceTuple{}}

	js := unit.NewUnitState("loaded", "active", "listening", &m1)
	js2 := unit.NewUnitState("loaded", "inactive", "dead", &m2)
	js3 := unit.NewUnitState("loaded", "inactive", "dead", &m2)
	js4 := unit.NewUnitState("loaded", "inactive", "dead", &m3)

	states := map[string]*unit.UnitState{"pong.service": js, "hello.service": js2, "echo.service": js3, "private.service": js4}
	machines := []machine.MachineState{m1, m2, m3}

	tr := registry.NewTestRegistry(machines, states, nil, nil, nil)
	registryCtl = &BlockedTestRegistry{echoAttempts, *tr}
}
