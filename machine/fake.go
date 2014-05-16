package machine

import (
	"reflect"
	"testing"
)

type FakeMachine struct {
	MachineState MachineState
}

func (fm *FakeMachine) State() MachineState {
	return fm.MachineState
}

func TestFakeMachine(t *testing.T) {
	ms := MachineState{ID: "XXX"}
	fm := FakeMachine{ms}

	ret := fm.State()
	if !reflect.DeepEqual(ms, ret) {
		t.Fatalf("FakeMachine.State() returned %v, expected %v", ret, ms)
	}
}
