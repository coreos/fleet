package main

import (
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
)

func TestListUnitScheduleFieldsToStrings(t *testing.T) {
	j := newTestJobFromUnitContents(t, "")
	for k, v := range map[string]string{
		"unit":     "foo.service",
		"dstate":   "inactive",
		"tmachine": "-",
		"state":    "-",
	} {
		f := listUnitScheduleFields[k](j, false)
		assertEqual(t, k, v, f)
	}

	f := listUnitScheduleFields["unit"](j, false)
	assertEqual(t, "unit", j.Name, f)

	j = newTestJobFromUnitContents(t, `[Unit]
Description=some description`)

	for _, state := range []job.JobState{job.JobStateLoaded, job.JobStateInactive, job.JobStateLaunched} {
		j.State = &state
		f := listUnitScheduleFields["state"](j, false)
		assertEqual(t, "state", string(state), f)
	}

	j.TargetMachineID = "some-id"
	ms := listUnitScheduleFields["tmachine"](j, true)
	assertEqual(t, "machine", "some-id", ms)

	j.TargetMachineID = "other-id"
	machineStates = map[string]*machine.MachineState{
		"other-id": &machine.MachineState{
			ID:       "other-id",
			PublicIP: "1.2.3.4",
		},
	}
	ms = listUnitScheduleFields["tmachine"](j, true)
	assertEqual(t, "machine", "other-id/1.2.3.4", ms)

}
