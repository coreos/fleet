package main

import (
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
)

func TestShowScheduleFieldsToStrings(t *testing.T) {
	su := job.ScheduledUnit{
		Name: "foo.service",
	}
	for k, v := range map[string]string{
		"unit":     "foo.service",
		"tmachine": "-",
		"state":    "-",
	} {
		f := showScheduleFields[k](su, false)
		assertEqual(t, k, v, f)
	}

	f := showScheduleFields["unit"](su, false)
	assertEqual(t, "unit", su.Name, f)

	for _, state := range []job.JobState{job.JobStateLoaded, job.JobStateInactive, job.JobStateLaunched} {
		su.State = &state
		f := showScheduleFields["state"](su, false)
		assertEqual(t, "state", string(state), f)
	}

	su.TargetMachineID = "some-id"
	ms := showScheduleFields["tmachine"](su, true)
	assertEqual(t, "machine", "some-id", ms)

	su.TargetMachineID = "other-id"
	machineStates = map[string]*machine.MachineState{
		"other-id": &machine.MachineState{
			ID:       "other-id",
			PublicIP: "1.2.3.4",
		},
	}
	ms = showScheduleFields["tmachine"](su, true)
	assertEqual(t, "machine", "other-id/1.2.3.4", ms)

}
