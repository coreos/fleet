package registry

import (
	"reflect"
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/unit"
)

func TestFakeRegistryUnitLifecycle(t *testing.T) {
	reg := NewFakeRegistry()

	units, err := reg.Units()
	if err != nil {
		t.Fatalf("Received error while calling Jobs: %v", err)
	}
	if !reflect.DeepEqual([]job.Unit{}, units) {
		t.Fatalf("Expected no units, got %v", units)
	}

	uf, _ := unit.NewUnitFile("")
	u1 := job.Unit{Name: "u1.service", Unit: *uf}
	err = reg.CreateUnit(&u1)
	if err != nil {
		t.Fatalf("Received error while calling CreateUnit: %v", err)
	}

	units, err = reg.Units()
	if err != nil {
		t.Fatalf("Received error while calling Units: %v", err)
	}
	if len(units) != 1 {
		t.Fatalf("Expected 1 Unit, got %v", units)
	}
	if units[0].Name != "u1.service" {
		t.Fatalf("Expected Job with name \"u1.service\", got %q", units[0].Name)
	}

	err = reg.ScheduleJob("u1.service", "XXX")
	if err != nil {
		t.Fatalf("Received error while calling ScheduleJob: %v", err)
	}

	su, err := reg.ScheduledUnit("u1.service")
	if err != nil {
		t.Fatalf("Received error while calling ScheduledUnit: %v", err)
	}
	if su.TargetMachineID != "XXX" {
		t.Fatalf("Unit should be scheduled to XXX, got %v", su.TargetMachineID)
	}

	err = reg.DestroyJob("u1.service")
	if err != nil {
		t.Fatalf("Received error while calling DestroyJob: %v", err)
	}

	units, err = reg.Units()
	if err != nil {
		t.Fatalf("Received error while calling Units: %v", err)
	}
	if !reflect.DeepEqual([]job.Unit{}, units) {
		t.Fatalf("Expected no units, got %v", units)
	}
}
