package main

import (
	"testing"
)

func TestListUnitFilesFieldsToStrings(t *testing.T) {
	j := newTestJobFromUnitContents(t, `[Unit]
Description=some description`)

	f := listUnitFilesFields["unit"](j, false)
	assertEqual(t, "unit", j.Name, f)

	uh := "f035b2f14edc4d23572e5f3d3d4cb4f78d0e53c3"
	fuh := listUnitFilesFields["hash"](j, true)
	suh := listUnitFilesFields["hash"](j, false)
	assertEqual(t, "hash", uh, fuh)
	assertEqual(t, "hash", uh[:7], suh)
}
