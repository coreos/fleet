package rpc

import (
	"testing"
	"time"

	pb "github.com/coreos/fleet/protobuf"
	"github.com/coreos/fleet/unit"
)

func TestInMemoryScheduleUnit(t *testing.T) {
	inmemoryRegistry := newInmemoryRegistry()

	scheduleUnit := &pb.ScheduledUnit{
		Name:         "foo",
		CurrentState: pb.TargetState_INACTIVE,
		MachineID:    "machine1",
	}
	inmemoryRegistry.scheduledUnits[scheduleUnit.Name] = *scheduleUnit
	contents := `
[Unit]
Description = Foo
`
	unitFile, err := unit.NewUnitFile(contents)
	if err != nil {
		t.Fatalf("unexpected error parsing unit %q: %v", contents, err)
	}
	unit := &pb.Unit{
		Name:         "foo",
		Unit:         unitFile.ToPB(),
		DesiredState: pb.TargetState_LOADED,
	}
	machineID := "testMachine"
	inmemoryRegistry.CreateUnit(unit)

	inmemoryRegistry.ScheduleUnit(unit.Name, machineID)

	unitsLen := len(inmemoryRegistry.Units())
	if unitsLen == 0 {
		t.Fatalf("unexpected amount of units in the in-memory registry got %d expected 1", unitsLen)
	}

	if !inmemoryRegistry.isScheduled(unit.Name, machineID) {
		t.Fatalf("unexpected error unit should be scheduled %s %s", unit.Name, machineID)
	}

	inmemoryRegistry.UnscheduleUnit(unit.Name, machineID)
	if inmemoryRegistry.isScheduled("foo", "testMachine") {
		t.Fatalf("unexpected error unit should NOT be scheduled %s %s", unit.Name, machineID)
	}

	if !inmemoryRegistry.DestroyUnit(unit.Name) {
		t.Fatalf("unexpected error unit have to be destroy %s", unit.Name)
	}

	unitsLen = len(inmemoryRegistry.Units())
	if unitsLen > 0 {
		t.Fatalf("unexpected amount of units in the in-memory registry got %d expected 0", unitsLen)
	}
}

func TestInMemoryUnitStates(t *testing.T) {
	inmemoryRegistry := newInmemoryRegistry()

	scheduleUnit := &pb.ScheduledUnit{
		Name:         "foo",
		CurrentState: pb.TargetState_INACTIVE,
		MachineID:    "machine1",
	}
	inmemoryRegistry.scheduledUnits[scheduleUnit.Name] = *scheduleUnit
	contents := `
[Unit]
Description = Foo
`
	unitFile, err := unit.NewUnitFile(contents)
	if err != nil {
		t.Fatalf("unexpected error parsing unit %q: %v", contents, err)
	}
	unit := &pb.Unit{
		Name:         "foo",
		Unit:         unitFile.ToPB(),
		DesiredState: pb.TargetState_LOADED,
	}
	machineID := "testMachine"
	ttl := 2 * time.Second
	inmemoryRegistry.CreateUnit(unit)
	inmemoryRegistry.ScheduleUnit(unit.Name, machineID)

	stateLoaded := &pb.UnitState{
		Name:        unit.Name,
		Hash:        "heh",
		LoadState:   "active",
		ActiveState: "loaded",
		SubState:    "active",
		MachineID:   machineID,
	}

	inmemoryRegistry.SaveUnitState(unit.Name, stateLoaded, ttl)
	if !inmemoryRegistry.isUnitLoaded(unit.Name, machineID) {
		u, ok := inmemoryRegistry.Unit(unit.Name)
		if !ok {
			t.Fatalf("unexpected error unit not found %s", unit.Name)
		}
		t.Fatalf("unexpected error unit expected to be loaded %v", u)
	}

	if len(inmemoryRegistry.UnitStates()) != 1 {
		t.Fatalf("unexpected amount of unit states in the in-memory registry got %d expected 1", len(inmemoryRegistry.UnitStates()))
	}
}
