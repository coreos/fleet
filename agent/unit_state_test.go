package agent

import (
	"reflect"
	"testing"

	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/unit"
)

func TestUpdateCache(t *testing.T) {
	name := "blah.service"
	mID := "mymachine"
	mach := &machine.FakeMachine{
		machine.MachineState{ID: mID},
	}
	us1 := &unit.UnitState{
		ActiveState: "active",
		UnitName:    name,
		MachineID:   mID,
	}
	us2 := &unit.UnitState{
		ActiveState: "inactive",
		UnitName:    name,
		MachineID:   mID,
	}
	ush1 := &unit.UnitStateHeartbeat{
		Name:  name,
		State: us1,
	}
	ush2 := &unit.UnitStateHeartbeat{
		Name:  name,
		State: us2,
	}
	ushN := &unit.UnitStateHeartbeat{
		Name:  name,
		State: nil,
	}

	for i, tt := range []struct {
		ush         *unit.UnitStateHeartbeat
		regStates   []unit.UnitState
		cacheStates map[string]*unit.UnitState
		want        []*unit.UnitState
	}{
		{
			// new heartbeat should be saved
			ush1,
			nil,
			make(map[string]*unit.UnitState),
			[]*unit.UnitState{us1},
		},
		{
			// new nil heartbeat should cause state to be removed from registry
			ushN,
			[]unit.UnitState{*us1},
			make(map[string]*unit.UnitState),
			[]*unit.UnitState{},
		},
		{
			// heartbeat different from one in cache should be saved
			ush2,
			nil,
			map[string]*unit.UnitState{"blah.service": us1},
			[]*unit.UnitState{us2},
		},
		{
			// heartbeat same as one already in cache shouldn't be saved
			ush1,
			nil,
			map[string]*unit.UnitState{"blah.service": us1},
			[]*unit.UnitState{},
		},
		{
			// nil heartbeat w/previous state should remove state from registry
			ushN,
			[]unit.UnitState{*us1},
			make(map[string]*unit.UnitState),
			[]*unit.UnitState{},
		},
	} {
		reg := registry.NewFakeRegistry()
		reg.SetUnitStates(tt.regStates)
		usp := NewUnitStatePublisher(reg, mach, 0)
		usp.cache = tt.cacheStates
		usp.updateCache(tt.ush)
		states, err := reg.UnitStates()
		if err != nil {
			t.Fatalf("case %d: error retrieving UnitStates: %v", i, err)
		}
		if !reflect.DeepEqual(states, tt.want) {
			t.Errorf("case %d: bad states: got %#v, want %#v", i, states, tt.want)
		}
	}
}
