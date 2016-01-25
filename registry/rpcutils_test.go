package registry

import (
	"reflect"
	"testing"

	"github.com/coreos/fleet/job"
	pb "github.com/coreos/fleet/protobuf"
	"github.com/coreos/fleet/unit"
)

const (
	TargetState_UNKNOWN pb.TargetState = 50
)

func TestRpcUnitStateToJobState(t *testing.T) {
	got := rpcUnitStateToJobState(pb.TargetState_INACTIVE)
	expect := job.JobStateInactive
	if !reflect.DeepEqual(got, expect) {
		t.Fatalf("got %#v, expected %#v", got, expect)
	}

	got = rpcUnitStateToJobState(pb.TargetState_LOADED)
	expect = job.JobStateLoaded
	if !reflect.DeepEqual(got, expect) {
		t.Fatalf("got %#v, expected %#v", got, expect)
	}

	got = rpcUnitStateToJobState(pb.TargetState_LAUNCHED)
	expect = job.JobStateLaunched
	if !reflect.DeepEqual(got, expect) {
		t.Fatalf("got %#v, expected %#v", got, expect)
	}

	// Unknown state
	got = rpcUnitStateToJobState(TargetState_UNKNOWN)
	expect = job.JobStateInactive
	if !reflect.DeepEqual(got, expect) {
		t.Fatalf("got %#v, expected %#v", got, expect)
	}
}

func TestRpcUnitStateToExtUnitState(t *testing.T) {
	want := &pb.UnitState{
		Name:        "foo",
		Hash:        "heh",
		LoadState:   "foo",
		ActiveState: "bar",
		SubState:    "baz",
		MachineID:   "machine1",
	}
	expect := &unit.UnitState{
		UnitName:    "foo",
		UnitHash:    "heh",
		LoadState:   "foo",
		ActiveState: "bar",
		SubState:    "baz",
		MachineID:   "machine1",
	}

	got := rpcUnitStateToExtUnitState(want)
	if !reflect.DeepEqual(got, expect) {
		t.Fatalf("got %#v, expected %#v", got, expect)
	}
}

func TestRpcUnitToJobUnit(t *testing.T) {
	contents := `
[Unit]
Description = Foo
`
	unitFile, err := unit.NewUnitFile(contents)
	if err != nil {
		t.Fatalf("unexpected error parsing unit %q: %v", contents, err)
	}

	want := &pb.Unit{
		Name:         "foo",
		Unit:         unitFile.ToPB(),
		DesiredState: pb.TargetState_LOADED,
	}
	expect := &job.Unit{
		Name:        "foo",
		Unit:        *unitFile,
		TargetState: job.JobStateLoaded,
	}

	got := rpcUnitToJobUnit(want)
	if !reflect.DeepEqual(got, expect) {
		t.Fatalf("got %#v, expected %#v", got, expect)
	}
}
