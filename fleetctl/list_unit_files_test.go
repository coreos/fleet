package main

import (
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/unit"
)

func newNamedTestJobFromUnitContents(t *testing.T, name, contents string) *job.Job {
	u, err := unit.NewUnit(contents)
	if err != nil {
		t.Fatalf("error creating Unit from %q: %v", contents, err)
	}
	j := job.NewJob(name, *u)
	if j == nil {
		t.Fatalf("error creating Job %q from %q", name, u)
	}
	return j
}

func newTestJobFromUnitContents(t *testing.T, contents string) *job.Job {
	return newNamedTestJobFromUnitContents(t, "foo.service", contents)
}

func TestListUnitFilesFieldsToStrings(t *testing.T) {
	j := newTestJobFromUnitContents(t, "")
	for k, v := range map[string]string{
		"hash": "da39a3e",
		"desc": "-",
	} {
		f := listUnitFilesFields[k](j, false)
		assertEqual(t, k, v, f)
	}

	f := listUnitFilesFields["unit"](j, false)
	assertEqual(t, "unit", j.Name, f)

	j = newTestJobFromUnitContents(t, `[Unit]
Description=some description`)
	d := listUnitFilesFields["desc"](j, false)
	assertEqual(t, "desc", "some description", d)

	uh := "f035b2f14edc4d23572e5f3d3d4cb4f78d0e53c3"
	fuh := listUnitFilesFields["hash"](j, true)
	suh := listUnitFilesFields["hash"](j, false)
	assertEqual(t, "hash", uh, fuh)
	assertEqual(t, "hash", uh[:7], suh)
}
