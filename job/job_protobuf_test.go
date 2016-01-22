package job

import (
	"reflect"
	"testing"

	pb "github.com/coreos/fleet/protobuf"
)

func TestScheduleUnitToPbScheduleUnit(t *testing.T) {
	jsInactive := JobStateInactive
	want := &ScheduledUnit{
		Name:            "foo",
		State:           &jsInactive,
		TargetMachineID: "machine1",
	}

	got := want.ToPB()
	expect := pb.ScheduledUnit{
		Name:         "foo",
		CurrentState: pb.TargetState_INACTIVE,
		MachineID:    "machine1",
	}
	if !reflect.DeepEqual(got, expect) {
		t.Fatalf("got %#v, expected %#v", got, expect)
	}
}

func TestJobStateToPbTargetState(t *testing.T) {
	expect := pb.TargetState_INACTIVE
	got := JobStateInactive.ToPB()
	if !reflect.DeepEqual(got, expect) {
		t.Fatalf("got %#v, expected %#v", got, expect)
	}
	expect = pb.TargetState_LOADED
	got = JobStateLoaded.ToPB()
	if !reflect.DeepEqual(got, expect) {
		t.Fatalf("got %#v, expected %#v", got, expect)
	}
	expect = pb.TargetState_LAUNCHED
	got = JobStateLaunched.ToPB()
	if !reflect.DeepEqual(got, expect) {
		t.Fatalf("got %#v, expected %#v", got, expect)
	}
}
