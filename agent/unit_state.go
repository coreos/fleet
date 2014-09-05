package agent

import (
	"encoding/json"
	"reflect"
	"sync"
	"time"

	"github.com/coreos/fleet/log"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/unit"
)

const numPublishers = 5

func NewUnitStatePublisher(reg registry.Registry, mach machine.Machine, ttl time.Duration) *UnitStatePublisher {
	return &UnitStatePublisher{
		reg:             reg,
		mach:            mach,
		ttl:             ttl,
		mutex:           sync.RWMutex{},
		cache:           make(map[string]*unit.UnitState),
		toPublish:       make(chan string),
		toPublishStates: make(map[string]*unit.UnitState),
	}
}

type UnitStatePublisher struct {
	reg  registry.Registry
	mach machine.Machine
	ttl  time.Duration

	mutex sync.RWMutex
	cache map[string]*unit.UnitState

	// toPublish is a queue indicating unit names for which a state publish event should occur.
	// It is possible for a unit name to end up in the queue for which a
	// state has already been published, in which case it triggers a no-op.
	toPublish chan string
	// toPublishStates is a mapping containing the latest UnitState which
	// should be published for each UnitName.
	toPublishStates map[string]*unit.UnitState
}

// Run caches all of the heartbeat objects from the provided channel, publishing
// them to the Registry every 5s. Heartbeat objects are also published as they
// are received on the channel.
func (p *UnitStatePublisher) Run(beatchan <-chan *unit.UnitStateHeartbeat, stop chan bool) {
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-time.After(p.ttl / 2):
				for name, us := range p.cache {
					p.queueForPublish(name, us)
				}
				p.pruneCache()
			}
		}
	}()

	machID := p.mach.State().ID

	l := &sync.RWMutex{}

	// Spawn goroutines to publish unit states.  Each goroutine waits until
	// it sees an event arrive on toPublish, then attempts to grab the
	// relevant UnitState and publish it to the registry.
	for i := 0; i < numPublishers; i++ {
		go func() {
			for {
				select {
				case <-stop:
					return
				case name := <-p.toPublish:
					l.Lock()
					// Grab the latest state by that name
					us, ok := p.toPublishStates[name]
					if !ok {
						// If one doesn't exist, ignore.
						l.Unlock()
						continue
					}
					delete(p.toPublishStates, name)
					l.Unlock()
					p.publishOne(name, us)

				}
			}
		}()
	}

	for {
		select {
		case <-stop:
			return
		case bt := <-beatchan:
			if bt.State != nil {
				bt.State.MachineID = machID
			}

			if p.updateCache(bt) {
				p.queueForPublish(bt.Name, bt.State)
			}
		}
	}
}

func (p *UnitStatePublisher) MarshalJSON() ([]byte, error) {
	p.mutex.Lock()
	data := struct {
		Cache map[string]*unit.UnitState
		Queue chan string
	}{
		Cache: p.cache,
		Queue: p.toPublish,
	}
	p.mutex.Unlock()

	return json.Marshal(data)
}

func (p *UnitStatePublisher) pruneCache() {
	for name, us := range p.cache {
		if us == nil {
			delete(p.cache, name)
		}
	}
}

func (p *UnitStatePublisher) publishOne(name string, us *unit.UnitState) {
	if us == nil {
		log.V(1).Infof("Destroying UnitState(%s) in Registry", name)
		err := p.reg.RemoveUnitState(name)
		if err != nil {
			log.Errorf("Failed to destroy UnitState(%s) in Registry: %v", name, err)
		}
	} else {
		// Sanity check - don't want to publish incomplete UnitStates
		// TODO(jonboulle): consider teasing apart a separate UnitState-like struct
		// so we can rely on a UnitState always being fully hydrated?

		// See https://github.com/coreos/fleet/issues/720
		//if len(us.UnitHash) == 0 {
		//	log.Errorf("Refusing to push UnitState(%s), no UnitHash: %#v", name, us)

		if len(us.MachineID) == 0 {
			log.Errorf("Refusing to push UnitState(%s), no MachineID: %#v", name, us)
		} else {
			log.V(1).Infof("Pushing UnitState(%s) to Registry: %#v", name, us)
			p.reg.SaveUnitState(name, us, p.ttl)
		}
	}
}

// queueForPublish notifies the publishing goroutines that a particular
// UnitState should be published to the Registry
func (p *UnitStatePublisher) queueForPublish(name string, us *unit.UnitState) {
	go func() {
		p.toPublishStates[name] = us
		// This may block for some time, but even if it occurs after
		// the above UnitState has already been published, it will
		// simply trigger a no-op
		p.toPublish <- name
	}()
}

// updateCache updates the cache of UnitStates which the UnitStatePublisher
// uses to determine when a change has occurred, and to do a periodic
// publishing of all UnitStates. It returns a boolean indicating whether the
// state in the given UnitStateHeartbeat differs from the state from the
// previous heartbeat of this unit, if any exists.
func (p *UnitStatePublisher) updateCache(update *unit.UnitStateHeartbeat) (changed bool) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	last, ok := p.cache[update.Name]
	p.cache[update.Name] = update.State

	if !ok || !reflect.DeepEqual(last, update.State) {
		changed = true
	}

	return
}

// Purge ensures that the UnitStates for all Units known in the
// UnitStatePublisher's cache are removed from the registry.
func (p *UnitStatePublisher) Purge() {
	for name := range p.cache {
		p.publishOne(name, nil)
	}
}
