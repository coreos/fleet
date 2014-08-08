package main

import (
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
)

func TestListUnitScheduleFieldsToStrings(t *testing.T) {
	su := job.ScheduledUnit{
		Name: "foo.service",
	}
	for k, v := range map[string]string{
		"unit":     "foo.service",
		"tmachine": "-",
		"state":    "-",
	} {
		f := listUnitScheduleFields[k](su, false)
		assertEqual(t, k, v, f)
	}

	f := listUnitScheduleFields["unit"](su, false)
	assertEqual(t, "unit", su.Name, f)

	for _, state := range []job.JobState{job.JobStateLoaded, job.JobStateInactive, job.JobStateLaunched} {
		su.State = &state
		f := listUnitScheduleFields["state"](su, false)
		assertEqual(t, "state", string(state), f)
	}

	su.TargetMachineID = "some-id"
	ms := listUnitScheduleFields["tmachine"](su, true)
	assertEqual(t, "machine", "some-id", ms)

	su.TargetMachineID = "other-id"
	machineStates = map[string]*machine.MachineState{
		"other-id": &machine.MachineState{
			ID:       "other-id",
			PublicIP: "1.2.3.4",
		},
	}
	ms = listUnitScheduleFields["tmachine"](su, true)
	assertEqual(t, "machine", "other-id/1.2.3.4", ms)

}
