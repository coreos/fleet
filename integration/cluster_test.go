package integration

import (
	"flag"
	"fmt"
	"reflect"
	"testing"

	"github.com/coreos/fleet/agent"
	"github.com/coreos/fleet/engine"
	"github.com/coreos/fleet/event"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/unit"
)

func init() {
	flag.CommandLine.Lookup("v").Value.Set("2")
	flag.CommandLine.Lookup("logtostderr").Value.Set("true")
}

func fakeAgent(ID string, reg registry.Registry, eBus *event.EventBus) (*agent.Agent, error) {
	fm := machine.FakeMachine{machine.MachineState{ID: ID}}
	fum := unit.NewFakeUnitManager()
	a, err := agent.New(fum, reg, &fm, agent.DefaultTTL, nil)
	if err != nil {
		return nil, err
	}

	eHandler := agent.NewEventHandler(a)
	eBus.AddListener(ID, eHandler)

	return a, nil
}

func fakeEngine(ID string, reg registry.Registry, eBus *event.EventBus) *engine.Engine {
	fm := machine.FakeMachine{machine.MachineState{ID: "E"}}
	e := engine.New(reg, &fm)
	eHandler := engine.NewEventHandler(e)
	eBus.AddListener("E", eHandler)
	return e
}

func TestIntegrationClusterJobConflicts(t *testing.T) {
	freg := registry.NewFakeRegistry()
	eBus := event.NewEventBus()

	_, err := fakeAgent("A1", freg, eBus)
	if err != nil {
		t.Fatalf("Failed creating new Agent: %v", err)
	}

	_, err = fakeAgent("A2", freg, eBus)
	if err != nil {
		t.Fatalf("Failed creating new Agent: %v", err)
	}

	_, err = fakeAgent("A3", freg, eBus)
	if err != nil {
		t.Fatalf("Failed creating new Agent: %v", err)
	}

	fakeEngine("E", freg, eBus)

	for i, _ := range []int{1, 2, 3} {
		name := fmt.Sprintf("J%d", i)
		j := job.NewJob(name, *unit.NewUnit("[X-Fleet]\nX-Conflicts=*"))

		freg.CreateJob(j)

		jo := job.NewOfferFromJob(*j, nil)
		eBus.Dispatch(&event.Event{"EventJobOffered", *jo, nil})

		bids, err := freg.Bids(jo)
		if err != nil {
			t.Fatalf("Received unexpected error fetching bids of job %s: %v", name, bids)
		}

		expect := []job.JobBid{
			job.JobBid{name, "A1"},
			job.JobBid{name, "A2"},
			job.JobBid{name, "A3"},
		}

		if !reflect.DeepEqual(expect, bids) {
			t.Errorf("Did not receive bids from all agents for job %s, got %v", name, bids)
		}
	}
}
