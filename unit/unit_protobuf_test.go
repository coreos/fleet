package unit

import (
	"reflect"
	"testing"

	pb "github.com/coreos/fleet/protobuf"
)

func TestUnitFileProtoBuf(t *testing.T) {
	u, err := NewUnitFile("[Service]\nExecStart=/bin/sleep 100\n")
	if err != nil {
		t.Fatalf("Unexpected error encountered creating unit: %v", err)
	}

	pbUnitFile := u.ToPB()
	if pbUnitFile.UnitOptions == nil {
		t.Fatalf("Unexpected error encountered creating a protobuf unit file")
	}

	contents := `
[Unit]
Description = Foo
`
	unitFile, err := NewUnitFile(contents)
	if err != nil {
		t.Fatalf("Unexpected error parsing unit %q: %v", contents, err)
	}

	pbUnitFile = unitFile.ToPB()
	if pbUnitFile.UnitOptions == nil {
		t.Fatal("Unexpected error encountered creating a protobuf unit file")
	}

	if pbUnitFile.UnitOptions[0].Section != "Unit" || pbUnitFile.UnitOptions[0].Name != "Description" || pbUnitFile.UnitOptions[0].Value != "Foo" {
		t.Errorf("Failed to persist data through protobuf unit: %v %v %v", pbUnitFile.UnitOptions[0].Value, pbUnitFile.UnitOptions[0].Name, pbUnitFile.UnitOptions[0].Section)
	}
}

func TestUnitStateProtoBuf(t *testing.T) {
	want := &UnitState{
		LoadState:   "foo",
		ActiveState: "bar",
		SubState:    "baz",
		MachineID:   "machine1",
		UnitHash:    "heh",
		UnitName:    "foo",
	}

	got := want.ToPB()
	if got == nil {
		t.Fatalf("Unexpected unit protobuf file to be nil")
	}
	expect := &pb.UnitState{
		Name:        "foo",
		Hash:        "heh",
		LoadState:   "foo",
		ActiveState: "bar",
		SubState:    "baz",
		MachineID:   "machine1",
	}
	if !reflect.DeepEqual(got, expect) {
		t.Fatalf("got %#v, expected %#v", got, expect)
	}
}
