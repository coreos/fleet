package main

import (
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/unit"
)

func newNamedTestJobUnitFromUnitContents(t *testing.T, name, contents string) job.JobUnit {
	u, err := unit.NewUnit(contents)
	if err != nil {
		t.Fatalf("error creating Unit from %q: %v", contents, err)
	}
	return job.JobUnit{
		Name: name,
		Unit: *u,
	}
}

func newTestJobUnitFromUnitContents(t *testing.T, contents string) job.JobUnit {
	return newNamedTestJobUnitFromUnitContents(t, "foo.service", contents)
}

func TestListUnitFilesFieldsToStrings(t *testing.T) {
	j := newTestJobUnitFromUnitContents(t, "")
	for k, v := range map[string]string{
		"hash": "da39a3e",
		"desc": "-",
	} {
		f := listUnitFilesFields[k](j, false)
		assertEqual(t, k, v, f)
	}

	f := listUnitFilesFields["unit"](j, false)
	assertEqual(t, "unit", j.Name, f)

	j = newTestJobUnitFromUnitContents(t, `[Unit]
Description=some description`)
	d := listUnitFilesFields["desc"](j, false)
	assertEqual(t, "desc", "some description", d)

	uh := "a0f275d46bc6ee0eca06be7c339913c07d99c0c7"
	fuh := listUnitFilesFields["hash"](j, true)
	suh := listUnitFilesFields["hash"](j, false)
	assertEqual(t, "hash", uh, fuh)
	assertEqual(t, "hash", uh[:7], suh)
}
