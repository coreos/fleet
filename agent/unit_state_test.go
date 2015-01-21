// Copyright 2014 CoreOS, Inc.
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

package agent

import (
	"encoding/json"
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/coreos/fleet/Godeps/_workspace/src/github.com/jonboulle/clockwork"

	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/unit"
)

func TestUpdateCache(t *testing.T) {
	name := "blah.service"
	mID := "mymachine"
	mach := &machine.FakeMachine{
		MachineState: machine.MachineState{ID: mID},
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
			MachineState: machine.MachineState{ID: "XXX"},
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

func TestDefaultPublisher(t *testing.T) {
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
		usp.publisher(tt.name, tt.state)
		us, err := freg.UnitStates()
		if err != nil {
			t.Errorf("case %d: unexpected error retrieving unit states: %v", i, err)
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

func TestUnitStatePublisherRunTiming(t *testing.T) {
	fclock := clockwork.NewFakeClock()
	states := make([]*unit.UnitState, 0)
	mu := sync.Mutex{} // protects states from concurrent modification
	published := make(chan struct{})
	pf := func(name string, us *unit.UnitState) {
		mu.Lock()
		states = append(states, us)
		mu.Unlock()
		go func() {
			published <- struct{}{}
		}()
	}
	usp := &UnitStatePublisher{
		mach:            &machine.FakeMachine{},
		ttl:             5 * time.Second,
		publisher:       pf,
		cache:           make(map[string]*unit.UnitState),
		cacheMutex:      sync.RWMutex{},
		toPublish:       make(chan string),
		toPublishStates: make(map[string]*unit.UnitState),
		toPublishMutex:  sync.RWMutex{},
		clock:           fclock,
	}
	usp.cache = map[string]*unit.UnitState{
		"foo.service": &unit.UnitState{
			UnitName:    "foo.service",
			ActiveState: "active",
			MachineID:   "XXX",
		},
	}

	bc := make(chan *unit.UnitStateHeartbeat)
	sc := make(chan struct{})
	go func() {
		usp.Run(bc, sc)
	}()

	// yield so Run() is definitely waiting on After()
	fclock.BlockUntil(1)

	// tick less than the publish interval
	fclock.Advance(time.Second)

	select {
	case <-published:
		t.Fatal("UnitState published unexpectedly!")
	default:
	}
	want := []*unit.UnitState{}
	mu.Lock()
	if !reflect.DeepEqual(states, want) {
		t.Errorf("bad UnitStates: got %#v, want %#v", states, want)
	}
	mu.Unlock()

	// now up to the publish interval
	fclock.Advance(4 * time.Second)
	want = []*unit.UnitState{
		&unit.UnitState{
			UnitName:    "foo.service",
			ActiveState: "active",
			MachineID:   "XXX",
		},
	}
	select {
	case <-published:
		mu.Lock()
		if !reflect.DeepEqual(states, want) {
			t.Errorf("bad UnitStates: got %#v, want %#v", states, want)
		}
		mu.Unlock()
	case <-time.After(time.Second):
		t.Fatalf("UnitState not published as expected!")
	}

	// reset states
	mu.Lock()
	states = []*unit.UnitState{}
	mu.Unlock()

	// yield so Run() is definitely waiting on After()
	fclock.BlockUntil(1)

	// tick less than the publish interval, again
	fclock.Advance(4 * time.Second)

	// no more should be published
	select {
	case <-published:
		t.Fatal("UnitState published unexpectedly!")
	default:
	}
	want = []*unit.UnitState{}
	mu.Lock()
	if !reflect.DeepEqual(states, want) {
		t.Errorf("bad UnitStates: got %#v, want %#v", states, want)
	}
	mu.Unlock()

	// tick way past the publish interval
	fclock.Advance(time.Hour)
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
		mu.Lock()
		if !reflect.DeepEqual(states, want) {
			t.Errorf("bad UnitStates: got %#v, want %#v", states, want)
		}
		mu.Unlock()
	case <-time.After(time.Second):
		t.Fatalf("UnitState not published as expected!")
	}

	// now stop the UnitStatePublisher
	close(sc)

	// reset states
	mu.Lock()
	states = []*unit.UnitState{}
	mu.Unlock()

	// yield so Run() is definitely waiting on After()
	fclock.BlockUntil(1)

	// tick way past the publish interval
	fclock.Advance(time.Hour)

	// no more states should be published
	select {
	case <-published:
		t.Fatal("UnitState published unexpectedly!")
	default:
	}
	want = []*unit.UnitState{}
	mu.Lock()
	if !reflect.DeepEqual(states, want) {
		t.Errorf("bad UnitStates: got %#v, want %#v", states, want)
	}
	mu.Unlock()
}

func TestUnitStatePublisherRunQueuing(t *testing.T) {
	states := make([]string, 0)
	mu := sync.Mutex{} // protects states from concurrent modification
	var wg sync.WaitGroup
	wg.Add(numPublishers)
	block := make(chan struct{})
	pf := func(name string, us *unit.UnitState) {
		wg.Done()
		<-block
		mu.Lock()
		states = append(states, name)
		mu.Unlock()
	}
	usp := &UnitStatePublisher{
		mach: &machine.FakeMachine{
			MachineState: machine.MachineState{
				ID: "some_id",
			},
		},
		ttl:             time.Hour, // we never expect to reach this
		publisher:       pf,
		cache:           make(map[string]*unit.UnitState),
		cacheMutex:      sync.RWMutex{},
		toPublish:       make(chan string),
		toPublishStates: make(map[string]*unit.UnitState),
		toPublishMutex:  sync.RWMutex{},
		clock:           clockwork.NewFakeClock(),
	}
	bc := make(chan *unit.UnitStateHeartbeat)
	sc := make(chan struct{})
	go func() {
		usp.Run(bc, sc)
	}()

	// first, fill up the publishers with a bunch of things
	wcache := make(map[string]*unit.UnitState)
	for i := 0; i < numPublishers; i++ {
		name := fmt.Sprintf("%d.service", i)
		us := &unit.UnitState{
			UnitName: name,
		}
		wcache[name] = us
		bc <- &unit.UnitStateHeartbeat{
			Name:  name,
			State: us,
		}
	}

	// wait until they're all started
	wg.Wait()

	// now the cache should contain them
	usp.cacheMutex.RLock()
	got := usp.cache
	if !reflect.DeepEqual(got, wcache) {
		t.Errorf("bad cache: got %#v, want %#v", got, wcache)
	}
	usp.cacheMutex.RUnlock()

	// as should the toPublish queue
	select {
	case update := <-usp.toPublish:
		t.Errorf("unexpected item in toPublish queue: %#v", update)
	default:
	}

	usp.cacheMutex.Lock()
	// flush the cache
	usp.cache = map[string]*unit.UnitState{
		"foo.service": &unit.UnitState{
			UnitName:    "foo.service",
			ActiveState: "active",
		},
	}
	usp.cacheMutex.Unlock()

	// now send a new UnitStateHeartbeat referencing something already in the cache
	bc <- &unit.UnitStateHeartbeat{
		Name: "foo.service",
		State: &unit.UnitState{
			UnitName:    "foo.service",
			ActiveState: "inactive",
		},
	}
	// since something changed, queue should be updated
	select {
	case got := <-usp.toPublish:
		want := "foo.service"
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("update not as expected: got %#v, want %#v", got, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("change not added to toPublish queue as expected!")
	}
	// as should the cache
	usp.cacheMutex.RLock()
	got = usp.cache
	wcache = map[string]*unit.UnitState{
		"foo.service": &unit.UnitState{
			UnitName:    "foo.service",
			ActiveState: "inactive",
			MachineID:   "some_id",
		},
	}
	if !reflect.DeepEqual(got, wcache) {
		t.Errorf("cache not as expected: got %#v, want %#v", got, wcache)
	}
	usp.cacheMutex.RUnlock()

	// finally, send another of the same update
	bc <- &unit.UnitStateHeartbeat{
		Name: "foo.service",
		State: &unit.UnitState{
			UnitName:    "foo.service",
			ActiveState: "inactive",
		},
	}
	// same state as cache, so queue should _not_ be updated, nor should cache
	select {
	case update := <-usp.toPublish:
		t.Errorf("unexpected change in toPublish queue: %#v", update)
	default:
	}
	usp.cacheMutex.RLock()
	got = usp.cache
	if !reflect.DeepEqual(got, wcache) {
		t.Errorf("cache not as expected: got %#v, want %#v", got, wcache)
	}
	usp.cacheMutex.RUnlock()
}

func TestUnitStatePublisherRunWithDelays(t *testing.T) {
	if runtime.GOMAXPROCS(-1) != 1 {
		t.Skipf("Broken for GOMAXPROCS != 1, currently %d", runtime.GOMAXPROCS(-1))
	}
	states := make([]string, 0)
	mu := sync.Mutex{} // protects states from concurrent modification
	fclock := clockwork.NewFakeClock()
	var wgs, wgf sync.WaitGroup // track starting and stopping of publishers
	slowpf := func(name string, us *unit.UnitState) {
		wgs.Done()
		// simulate a delay in communication with the registry
		fclock.Sleep(3 * time.Second)
		mu.Lock()
		states = append(states, name)
		mu.Unlock()
		wgf.Done()
	}

	usp := &UnitStatePublisher{
		mach:            &machine.FakeMachine{},
		ttl:             time.Hour, // we never expect to reach this
		publisher:       slowpf,
		cache:           make(map[string]*unit.UnitState),
		cacheMutex:      sync.RWMutex{},
		toPublish:       make(chan string),
		toPublishStates: make(map[string]*unit.UnitState),
		toPublishMutex:  sync.RWMutex{},
		clock:           clockwork.NewFakeClock(),
	}

	bc := make(chan *unit.UnitStateHeartbeat)
	sc := make(chan struct{})

	wgs.Add(numPublishers)
	wgf.Add(numPublishers)

	go func() {
		usp.Run(bc, sc)
	}()

	// queue a bunch of unit states for publishing - occupy all publish workers
	wantPublished := make([]string, numPublishers)
	for i := 0; i < numPublishers; i++ {
		name := fmt.Sprintf("%d.service", i)
		wantPublished[i] = name
		usp.queueForPublish(
			name,
			&unit.UnitState{
				UnitName: name,
			},
		)
	}

	// now queue some more unit states for publishing - expect them to hang
	go usp.queueForPublish("foo.service", &unit.UnitState{UnitName: "foo.service", ActiveState: "active"})
	go usp.queueForPublish("bar.service", &unit.UnitState{})
	go usp.queueForPublish("baz.service", &unit.UnitState{})

	// re-queue one of the states; this should replace the above
	go usp.queueForPublish("foo.service", &unit.UnitState{UnitName: "foo.service", ActiveState: "inactive"})

	// wait for all publish workers to start
	wgs.Wait()

	// so far, no states should be published, and the last three should be enqueued
	ws := []string{}
	mu.Lock()
	if !reflect.DeepEqual(states, ws) {
		t.Errorf("bad UnitStates: got %#v, want %#v", states, ws)
	}
	mu.Unlock()

	wtps := map[string]*unit.UnitState{
		"foo.service": &unit.UnitState{
			UnitName:    "foo.service",
			ActiveState: "inactive",
		},
		"bar.service": &unit.UnitState{},
		"baz.service": &unit.UnitState{},
	}

	usp.toPublishMutex.RLock()
	if !reflect.DeepEqual(usp.toPublishStates, wtps) {
		t.Errorf("bad toPublishStates")
		t.Logf("got:\n")
		for k, v := range usp.toPublishStates {
			t.Logf("  %v = %#v", k, v)
		}
		t.Logf("want:\n")
		for k, v := range wtps {
			t.Logf("  %v = %#v", k, v)
		}
	}
	usp.toPublishMutex.RUnlock()

	// end the registry delay by ticking past it just once -
	// expect three more publishers to start, and block
	wgs.Add(3)
	fclock.Advance(3 * time.Second)

	// wait for the original publishers to finish
	wgf.Wait()

	// now, all five original states should have been published
	ws = wantPublished
	mu.Lock()
	sort.Strings(states)
	if !reflect.DeepEqual(states, ws) {
		t.Errorf("bad published UnitStates: got %#v, want %#v", states, ws)
	}
	mu.Unlock()

	// wait for the new publishers to start
	wgs.Wait()

	// and the other states should be flushed from the toPublish queue
	wtps = map[string]*unit.UnitState{}
	usp.toPublishMutex.RLock()
	if !reflect.DeepEqual(usp.toPublishStates, wtps) {
		t.Errorf("bad toPublishStates: got %#v, want %#v", usp.toPublishStates, wtps)
	}
	usp.toPublishMutex.RUnlock()

	// but still not published
	mu.Lock()
	sort.Strings(states)
	if !reflect.DeepEqual(states, ws) {
		t.Errorf("bad published UnitStates: got %#v, want %#v", states, ws)
	}

	// clear the published states
	states = []string{}
	mu.Unlock()

	// prepare for the new publishers
	wgf.Add(3)

	// tick past the registry delay again so the new publishers continue
	fclock.Advance(10 * time.Second)

	// wait for them to complete
	wgf.Wait()

	// and now the other states should be published
	ws = []string{"bar.service", "baz.service", "foo.service"}
	mu.Lock()
	sort.Strings(states)
	if !reflect.DeepEqual(states, ws) {
		t.Errorf("bad UnitStates: got %#v, want %#v", states, ws)
	}
	mu.Unlock()

	// and toPublish queue should still be empty
	wtps = map[string]*unit.UnitState{}
	usp.toPublishMutex.RLock()
	if !reflect.DeepEqual(usp.toPublishStates, wtps) {
		t.Errorf("bad toPublishStates: got %#v, want %#v", usp.toPublishStates, wtps)
	}
	usp.toPublishMutex.RUnlock()
}

func TestQueueForPublish(t *testing.T) {
	usp := &UnitStatePublisher{
		toPublish:       make(chan string),
		toPublishStates: make(map[string]*unit.UnitState),
	}
	go usp.queueForPublish("foo.service", &unit.UnitState{
		UnitName:    "foo.service",
		ActiveState: "active",
	})
	select {
	case name := <-usp.toPublish:
		if name != "foo.service" {
			t.Errorf("unexpected name on toPublish channel: %v", name)
		}
	case <-time.After(time.Second):
		t.Fatal("did not receive on toPublish channel as expected")
	}
	want := map[string]*unit.UnitState{
		"foo.service": &unit.UnitState{
			UnitName:    "foo.service",
			ActiveState: "active",
		},
	}
	if !reflect.DeepEqual(want, usp.toPublishStates) {
		t.Errorf("bad toPublishStates: want %#v, got %#v", want, usp.toPublishStates)
	}
}

func TestMarshalJSON(t *testing.T) {
	usp := NewUnitStatePublisher(&registry.FakeRegistry{}, &machine.FakeMachine{}, 0)
	got, err := json.Marshal(usp)
	if err != nil {
		t.Fatalf("unexpected error marshalling: %#v", err)
	}
	want := `{"Cache":{},"ToPublish":{}}`
	if string(got) != want {
		t.Fatalf("Bad JSON representation: got\n%s\n\nwant\n%s", string(got), want)
	}

	usp = NewUnitStatePublisher(&registry.FakeRegistry{}, &machine.FakeMachine{}, 0)
	usp.cache = map[string]*unit.UnitState{
		"foo.service": &unit.UnitState{
			UnitName:    "foo.service",
			ActiveState: "active",
			MachineID:   "asdf",
		},
		"bar.service": &unit.UnitState{
			UnitName:    "bar.service",
			ActiveState: "inactive",
			MachineID:   "asdf",
		},
	}
	usp.toPublishStates = map[string]*unit.UnitState{
		"woof.service": &unit.UnitState{
			UnitName:    "woof.service",
			ActiveState: "active",
			MachineID:   "asdf",
		},
	}
	got, err = json.Marshal(usp)
	if err != nil {
		t.Fatalf("unexpected error marshalling: %v", err)
	}
	want = `{"Cache":{"bar.service":{"LoadState":"","ActiveState":"inactive","SubState":"","MachineID":"asdf","UnitHash":"","UnitName":"bar.service"},"foo.service":{"LoadState":"","ActiveState":"active","SubState":"","MachineID":"asdf","UnitHash":"","UnitName":"foo.service"}},"ToPublish":{"woof.service":{"LoadState":"","ActiveState":"active","SubState":"","MachineID":"asdf","UnitHash":"","UnitName":"woof.service"}}}`
	if string(got) != want {
		t.Fatalf("Bad JSON representation: got\n%s\n\nwant\n%s", string(got), want)
	}

}
