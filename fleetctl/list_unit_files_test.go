package main

import (
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/unit"
)

func newNamedTestUnitFromUnitContents(t *testing.T, name, contents string) job.Unit {
	u, err := unit.NewUnitFile(contents)
	if err != nil {
		t.Fatalf("error creating Unit from %q: %v", contents, err)
	}
	return job.Unit{
		Name: name,
		Unit: *u,
	}
}

func newTestUnitFromUnitContents(t *testing.T, contents string) job.Unit {
	return newNamedTestUnitFromUnitContents(t, "foo.service", contents)
}

func TestListUnitFilesFieldsToStrings(t *testing.T) {
	j := newTestUnitFromUnitContents(t, "")
	su := &job.ScheduledUnit{
		Name: "foo.service",
	}
	for k, v := range map[string]string{
		"hash":     "da39a3e",
		"desc":     "-",
		"dstate":   "-",
		"tmachine": "-",
		"state":    "-",
	} {
		f := listUnitFilesFields[k](j, su, false)
		assertEqual(t, k, v, f)
	}

	f := listUnitFilesFields["unit"](j, su, false)
	assertEqual(t, "unit", j.Name, f)

	j = newTestUnitFromUnitContents(t, `[Unit]
Description=some description`)
	d := listUnitFilesFields["desc"](j, su, false)
	assertEqual(t, "desc", "some description", d)

	for _, state := range []job.JobState{job.JobStateLoaded, job.JobStateInactive, job.JobStateLaunched} {
		su.State = &state
		f := listUnitFilesFields["state"](j, su, false)
		assertEqual(t, "state", string(state), f)
	}

	su.TargetMachineID = "some-id"
	ms := listUnitFilesFields["tmachine"](j, su, true)
	assertEqual(t, "machine", "some-id", ms)

	su.TargetMachineID = "other-id"
	machineStates = map[string]*machine.MachineState{
		"other-id": &machine.MachineState{
			ID:       "other-id",
			PublicIP: "1.2.3.4",
		},
	}
	ms = listUnitFilesFields["tmachine"](j, su, true)
	assertEqual(t, "machine", "other-id/1.2.3.4", ms)

	uh := "a0f275d46bc6ee0eca06be7c339913c07d99c0c7"
	fuh := listUnitFilesFields["hash"](j, su, true)
	suh := listUnitFilesFields["hash"](j, su, false)
	assertEqual(t, "hash", uh, fuh)
	assertEqual(t, "hash", uh[:7], suh)
}
