package client

import (
	"reflect"
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/schema"
	"github.com/coreos/fleet/unit"
)

func newUnit(t *testing.T, str string) unit.Unit {
	u, err := unit.NewUnit(str)
	if err != nil {
		t.Fatalf("Unexpected error creating unit from %q: %v", str, err)
	}
	return *u
}

func TestMapUnitEntityToJob(t *testing.T) {
	loaded := job.JobStateLoaded
	inactive := job.JobStateInactive

	tests := []struct {
		input  schema.Unit
		expect *job.Job
	}{
		{
			schema.Unit{
				Name:         "XXX",
				CurrentState: "loaded",
				DesiredState: "inactive",
				Systemd: &schema.SystemdState{
					LoadState:   "loaded",
					ActiveState: "active",
					SubState:    "running",
				},
				Options: []*schema.UnitOption{
					&schema.UnitOption{Section: "Service", Name: "ExecStart", Value: "/usr/bin/sleep 3000"},
				},
			},
			&job.Job{
				Name:        "XXX",
				State:       &loaded,
				TargetState: inactive,
				Unit:        newUnit(t, "[Service]\nExecStart=/usr/bin/sleep 3000\n"),
				UnitState: &unit.UnitState{
					UnitName:    "XXX",
					LoadState:   "loaded",
					ActiveState: "active",
					SubState:    "running",
				},
			},
		},

		// Lack of LoadState should result in a nil UnitState
		{
			schema.Unit{
				Name:         "XXX",
				CurrentState: "loaded",
				DesiredState: "loaded",
				Options: []*schema.UnitOption{
					&schema.UnitOption{Section: "Service", Name: "ExecStart", Value: "/usr/bin/sleep 3000"},
				},
			},
			&job.Job{
				Name:        "XXX",
				State:       &loaded,
				TargetState: loaded,
				Unit:        newUnit(t, "[Service]\nExecStart=/usr/bin/sleep 3000\n"),
			},
		},
	}

	for i, tt := range tests {
		output, err := mapUnitToJob(&tt.input, nil)
		if err != nil {
			t.Errorf("case %d: err=%v", i, err)
			continue
		}
		if !reflect.DeepEqual(tt.expect, output) {
			t.Errorf("case %d: expect=%v, got=%v", i, tt.expect, *output)
		}
	}
}

func TestMapUnitEntityToJobMachineFields(t *testing.T) {
	tests := []struct {
		input  schema.Unit
		expect *job.Job
	}{
		{
			schema.Unit{
				Systemd: &schema.SystemdState{
					LoadState:   "loaded",
					ActiveState: "active",
					SubState:    "running",
					MachineID:   "YYY",
				},
			},
			&job.Job{
				UnitState: &unit.UnitState{
					LoadState:   "loaded",
					ActiveState: "active",
					SubState:    "running",
					MachineID:   "YYY",
				},
			},
		},

		// Missing MachineState in map does not result in loss of Machine ID
		{
			schema.Unit{
				Systemd: &schema.SystemdState{
					LoadState:   "loaded",
					ActiveState: "active",
					SubState:    "running",
					MachineID:   "FFF",
				},
			},
			&job.Job{
				UnitState: &unit.UnitState{
					LoadState:   "loaded",
					ActiveState: "active",
					SubState:    "running",
					MachineID:   "FFF",
				},
			},
		},
	}

	mm := map[string]*machine.MachineState{
		"YYY": &machine.MachineState{ID: "YYY", PublicIP: "ZZZ"},
	}

	for i, tt := range tests {
		output, err := mapUnitToJob(&tt.input, mm)
		if err != nil {
			t.Errorf("case %d: err=%v", i, err)
			continue
		}
		if !reflect.DeepEqual(tt.expect.UnitState, output.UnitState) {
			t.Errorf("case %d: expect=%v, got=%v", i, tt.expect.UnitState, output.UnitState)
		}
	}
}
