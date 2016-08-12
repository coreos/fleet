// Copyright 2016 The fleet Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
