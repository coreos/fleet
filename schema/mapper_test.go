package schema

import (
	"testing"

	"github.com/coreos/fleet/job"
)

func TestMapJobToSchema(t *testing.T) {
	loaded := job.JobStateLoaded

	tests := []struct {
		input  job.Job
		expect Unit
	}{
		{
			job.Job{
				Name:            "XXX",
				State:           &loaded,
				TargetState:     job.JobStateLaunched,
				TargetMachineID: "ZZZ",
				Unit:            newUnit(t, "[Service]\nExecStart=/usr/bin/sleep 3000\n"),
				UnitState: &unit.UnitState{
					LoadState:   "loaded",
					ActiveState: "active",
					SubState:    "running",
					MachineID:   "YYY",
				},
			},
			Unit{
				Name:            "XXX",
				CurrentState:    "loaded",
				DesiredState:    "launched",
				TargetMachineID: "ZZZ",
				Systemd: &SystemdState{
					LoadState:   "loaded",
					ActiveState: "active",
					SubState:    "running",
					MachineID:   "YYY",
				},
				Options: []*UnitOption{
					&UnitOption{Section: "Service", Name: "ExecStart", Value: "/usr/bin/sleep 3000"},
				},
			},
		},
	}

	for i, tt := range tests {
		output, err := mapJobToSchema(&tt.input)
		if err != nil {
			t.Errorf("case %d: call to mapJobToSchema failed: %v", i, err)
			continue
		}
		if !reflect.DeepEqual(tt.expect, *output) {
			t.Errorf("case %d: expect=%v, got=%v", i, tt.expect, *output)
		}
	}
}

func TestMapSchemaToJob(t *testing.T) {
	loaded := job.JobStateLoaded

	tests := []struct {
		input  Unit
		expect *job.Job
	}{
		{
			Unit{
				Name:         "XXX",
				CurrentState: "loaded",
				DesiredState: "inactive",
				Systemd: &SystemdState{
					LoadState:   "loaded",
					ActiveState: "active",
					SubState:    "running",
				},
				Options: []*UnitOption{
					&UnitOption{Section: "Service", Name: "ExecStart", Value: "/usr/bin/sleep 3000"},
				},
			},
			&job.Job{
				Name:  "XXX",
				State: &loaded,
				TargetState: job.JobStateInactive,
				Unit:  newUnit(t, "[Service]\nExecStart=/usr/bin/sleep 3000\n"),
				UnitState: &unit.UnitState{
					LoadState:   "loaded",
					ActiveState: "active",
					SubState:    "running",
				},
			},
		},

		// Lack of LoadState should result in a nil UnitState
		{
			Unit{
				Name:         "XXX",
				CurrentState: "loaded",
				DesiredState: "inactive",
				Options: []*UnitOption{
					&UnitOption{Section: "Service", Name: "ExecStart", Value: "/usr/bin/sleep 3000"},
				},
			},
			&job.Job{
				Name:  "XXX",
				State: &loaded,
				TargetState: job.JobStateInactive,
				Unit:  newUnit(t, "[Service]\nExecStart=/usr/bin/sleep 3000\n"),
			},
		},
	}

	for i, tt := range tests {
		output, err := MapSchemaToJob(&tt.input, nil)
		if err != nil {
			t.Errorf("case %d: err=%v", i, err)
			continue
		}
		if !reflect.DeepEqual(tt.expect, output) {
			t.Errorf("case %d: expect=%v, got=%v", i, tt.expect, *output)
		}
	}
}
