package agent

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/pkg"
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
	ushNil := &unit.UnitStateHeartbeat{
		Name:  name,
		State: nil,
	}

	tests := []struct {
		ush         *unit.UnitStateHeartbeat
		cacheBefore map[string]*unit.UnitState
		cacheAfter  map[string]*unit.UnitState
		changed     bool
	}{
		{
			// new heartbeat should be saved
			ush1,
			map[string]*unit.UnitState{},
			map[string]*unit.UnitState{ush1.Name: us1},
			true,
		},
		{
			// nil heartbeat should not remove state from cache
			ushNil,
			map[string]*unit.UnitState{ush1.Name: us1},
			map[string]*unit.UnitState{ush1.Name: nil},
			true,
		},
		{
			// heartbeat different from one in cache should be saved
			ush2,
			map[string]*unit.UnitState{ush2.Name: us1},
			map[string]*unit.UnitState{ush2.Name: us2},
			true,
		},
		{
			// heartbeat same as one already in cache shouldn't be saved
			ush1,
			map[string]*unit.UnitState{ush1.Name: us1},
			map[string]*unit.UnitState{ush1.Name: us1},
			false,
		},
		{
			// non-nil heartbeat should overwrite existing nil
			ush1,
			map[string]*unit.UnitState{ush1.Name: nil},
			map[string]*unit.UnitState{ush1.Name: us1},
			true,
		},
		{
			// nil heartbeat should not overwrite existing nil
			ushNil,
			map[string]*unit.UnitState{ush1.Name: nil},
			map[string]*unit.UnitState{ush1.Name: nil},
			false,
		},
	}

	for i, tt := range tests {
		usp := NewUnitStatePublisher(nil, mach, 0)
		usp.cache = tt.cacheBefore
		changed := usp.updateCache(tt.ush)
		if tt.changed != changed {
			t.Errorf("case %d: expected changed=%t, got %t", i, tt.changed, changed)
		}
		if !reflect.DeepEqual(tt.cacheAfter, usp.cache) {
			t.Errorf("case %d: expected cache after operation %#v, got %#v", i, tt.cacheAfter, usp.cache)
		}
	}
}

func TestPruneCache(t *testing.T) {
	tests := []struct {
		cacheBefore map[string]*unit.UnitState
		cacheAfter  map[string]*unit.UnitState
	}{
		{
			cacheBefore: map[string]*unit.UnitState{
				"foo.service": &unit.UnitState{},
			},
			cacheAfter: map[string]*unit.UnitState{
				"foo.service": &unit.UnitState{},
			},
		},

		{
			cacheBefore: map[string]*unit.UnitState{
				"foo.service": nil,
			},
			cacheAfter: map[string]*unit.UnitState{},
		},
		{
			cacheBefore: map[string]*unit.UnitState{
				"foo.service": &unit.UnitState{},
				"bar.service": nil,
			},
			cacheAfter: map[string]*unit.UnitState{
				"foo.service": &unit.UnitState{},
			},
		},
	}

	for i, tt := range tests {
		mach := &machine.FakeMachine{
			machine.MachineState{ID: "XXX"},
		}
		usp := NewUnitStatePublisher(nil, mach, 0)
		usp.cache = tt.cacheBefore
		usp.pruneCache()
		if !reflect.DeepEqual(tt.cacheAfter, usp.cache) {
			t.Errorf("case %d: expected cache after operation %#v, got %#v", i, tt.cacheAfter, usp.cache)
		}
	}
}

func TestPurge(t *testing.T) {
	cache := map[string]*unit.UnitState{
		"foo.service": &unit.UnitState{
			UnitName:    "foo.service",
			ActiveState: "loaded",
		},
		"bar.service": nil,
	}
	initStates := []unit.UnitState{
		unit.UnitState{
			UnitName:    "foo.service",
			ActiveState: "active",
		},
		unit.UnitState{
			UnitName:    "bar.service",
			ActiveState: "loaded",
		},
		unit.UnitState{
			UnitName:    "baz.service",
			ActiveState: "inactive",
		},
	}
	want := []*unit.UnitState{
		&unit.UnitState{
			UnitName:    "baz.service",
			ActiveState: "inactive",
		},
	}
	freg := registry.NewFakeRegistry()
	freg.SetUnitStates(initStates)
	usp := NewUnitStatePublisher(freg, &machine.FakeMachine{}, 0)
	usp.cache = cache

	usp.Purge()

	us, err := freg.UnitStates()
	if err != nil {
		t.Errorf("unexpected error retrieving unit states: %v", err)
	} else if !reflect.DeepEqual(us, want) {
		msg := fmt.Sprintln("received unexpected unit states")
		msg += fmt.Sprintf("got: %#v\n", us)
		msg += fmt.Sprintf("want: %#v\n", want)
		t.Error(msg)
	}
}

func TestPublishOne(t *testing.T) {
	testCases := []struct {
		name       string
		state      *unit.UnitState
		initStates []unit.UnitState
		want       []*unit.UnitState
	}{
		// Simplest case - success
		{
			"foo.service",
			&unit.UnitState{
				UnitName:    "foo.service",
				ActiveState: "active",
				MachineID:   "xyz",
			},
			nil,
			[]*unit.UnitState{
				&unit.UnitState{
					UnitName:    "foo.service",
					ActiveState: "active",
					MachineID:   "xyz",
				},
			},
		},
		// Unit state with no machine ID is refused
		{
			"foo.service",
			&unit.UnitState{
				UnitName:    "foo.service",
				ActiveState: "active",
			},
			nil,
			[]*unit.UnitState{},
		},
		// Destroying existing unit state
		{
			"foo.service",
			nil,
			[]unit.UnitState{
				unit.UnitState{
					UnitName:    "foo.service",
					ActiveState: "active",
				},
			},
			[]*unit.UnitState{},
		},
		// Destroying non-existent unit state should not fail
		{
			"foo.service",
			nil,
			[]unit.UnitState{},
			[]*unit.UnitState{},
		},
	}

	for i, tt := range testCases {
		freg := registry.NewFakeRegistry()
		freg.SetUnitStates(tt.initStates)
		usp := NewUnitStatePublisher(freg, &machine.FakeMachine{}, 0)
		usp.publishOne(tt.name, tt.state)
		us, err := freg.UnitStates()
		if err != nil {
			t.Errorf("case %d: unexpected error retrieving unit states: %v", err)
			continue
		}
		if !reflect.DeepEqual(us, tt.want) {
			msg := fmt.Sprintf("case %d: received unexpected unit states\n", i)
			msg += fmt.Sprintf("got: %#v\n", us)
			msg += fmt.Sprintf("want: %#v\n", tt.want)
			t.Error(msg)
		}
	}
}

func TestUnitStatePublisherRun(t *testing.T) {
	fclock := &pkg.FakeClock{}
	states := make([]*unit.UnitState, 0)
	published := make(chan struct{})
	pf := func(name string, us *unit.UnitState) {
		states = append(states, us)
		go func() {
			published <- struct{}{}
		}()
	}
	usp := &UnitStatePublisher{
		mach:      &machine.FakeMachine{},
		publisher: pf,
		ttl:       5 * time.Second,
		mutex:     sync.RWMutex{},
		cache:     make(map[string]*unit.UnitState),
		clock:     fclock,
	}
	usp.cache = map[string]*unit.UnitState{
		"foo.service": &unit.UnitState{
			UnitName:    "foo.service",
			ActiveState: "active",
			MachineID:   "XXX",
		},
	}

	bc := make(chan *unit.UnitStateHeartbeat)
	sc := make(chan bool)
	go func() {
		usp.Run(bc, sc)
	}()

	// block until Run() is definitely waiting on After()
	// TODO(jonboulle): do this more elegantly!!!
	for {
		if fclock.Sleepers() == 1 {
			break
		}
	}

	// tick less than the publish interval
	fclock.Tick(time.Second)

	select {
	case <-published:
		t.Fatal("UnitState published unexpectedly!")
	default:
	}
	want := []*unit.UnitState{}
	if !reflect.DeepEqual(states, want) {
		t.Errorf("bad UnitStates: got %#v, want %#v", states, want)
	}

	// now up to the publish interval
	fclock.Tick(4 * time.Second)
	want = []*unit.UnitState{
		&unit.UnitState{
			UnitName:    "foo.service",
			ActiveState: "active",
			MachineID:   "XXX",
		},
	}
	select {
	case <-published:
		if !reflect.DeepEqual(states, want) {
			t.Errorf("bad UnitStates: got %#v, want %#v", states, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("UnitState not published as expected!")
	}

	// reset states
	states = []*unit.UnitState{}

	// block until Run() is definitely waiting on After()
	// TODO(jonboulle): do this more elegantly!!!
	for {
		if fclock.Sleepers() == 1 {
			break
		}
	}

	// tick less than the publish interval, again
	fclock.Tick(4 * time.Second)

	// no more should be published
	select {
	case <-published:
		t.Fatal("UnitState published unexpectedly!")
	default:
	}
	want = []*unit.UnitState{}
	if !reflect.DeepEqual(states, want) {
		t.Errorf("bad UnitStates: got %#v, want %#v", states, want)
	}

	// tick way past the publish interval
	fclock.Tick(time.Hour)
	want = []*unit.UnitState{
		&unit.UnitState{
			UnitName:    "foo.service",
			ActiveState: "active",
			MachineID:   "XXX",
		},
	}

	// state should be published, but just once (since it's still just a single event)
	select {
	case <-published:
		if !reflect.DeepEqual(states, want) {
			t.Errorf("bad UnitStates: got %#v, want %#v", states, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("UnitState not published as expected!")
	}

	// now stop the UnitStatePublisher
	close(sc)

	// reset states
	states = []*unit.UnitState{}

	// block until Run() is definitely waiting on After()
	// TODO(jonboulle): do this more elegantly!!!
	for {
		if fclock.Sleepers() == 1 {
			break
		}
	}

	// tick way past the publish interval
	fclock.Tick(time.Hour)

	// no more states should be published
	select {
	case <-published:
		t.Fatal("UnitState published unexpectedly!")
	default:
	}
	want = []*unit.UnitState{}
	if !reflect.DeepEqual(states, want) {
		t.Errorf("bad UnitStates: got %#v, want %#v", states, want)
	}
}
