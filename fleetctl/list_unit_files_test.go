package main

import (
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/schema"
)

func TestListUnitFilesFieldsToStrings(t *testing.T) {
	u := schema.Unit{
		Name:    "foo.service",
		Options: []*schema.UnitOption{},
	}

	for k, v := range map[string]string{
		"hash":     "da39a3e",
		"desc":     "-",
		"dstate":   "-",
		"tmachine": "-",
		"state":    "-",
	} {
		f := listUnitFilesFields[k](u, false)
		assertEqual(t, k, v, f)
	}

	f := listUnitFilesFields["unit"](u, false)
	assertEqual(t, "unit", u.Name, f)

	u = schema.Unit{
		Name: "foo.service",
		Options: []*schema.UnitOption{
			&schema.UnitOption{Section: "Unit", Name: "Description", Value: "some description"},
		},
	}

	d := listUnitFilesFields["desc"](u, false)
	assertEqual(t, "desc", "some description", d)

	for _, state := range []job.JobState{job.JobStateLoaded, job.JobStateInactive, job.JobStateLaunched} {
		u.CurrentState = string(state)
		f := listUnitFilesFields["state"](u, false)
		assertEqual(t, "state", string(state), f)
	}

	// machineStates must be initialized since cAPI is not set
	machineStates = map[string]*machine.MachineState{}

	u.Machine = "some-id"
	ms := listUnitFilesFields["tmachine"](u, true)
	assertEqual(t, "machine", "some-id", ms)

	u.Machine = "other-id"
	machineStates = map[string]*machine.MachineState{
		"other-id": &machine.MachineState{
			ID:       "other-id",
			PublicIP: "1.2.3.4",
		},
	}
	ms = listUnitFilesFields["tmachine"](u, true)
	assertEqual(t, "machine", "other-id/1.2.3.4", ms)

	uh := "a0f275d46bc6ee0eca06be7c339913c07d99c0c7"
	fuh := listUnitFilesFields["hash"](u, true)
	suh := listUnitFilesFields["hash"](u, false)
	assertEqual(t, "hash", uh, fuh)
	assertEqual(t, "hash", uh[:7], suh)
}
