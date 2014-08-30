package agent

import (
	"reflect"
	"sync"
	"time"

	"github.com/coreos/fleet/log"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/unit"
)

func NewUnitStatePublisher(reg registry.Registry, mach machine.Machine, ttl time.Duration) *UnitStatePublisher {
	return &UnitStatePublisher{
		reg:   reg,
		mach:  mach,
		ttl:   ttl,
		mutex: sync.RWMutex{},
		cache: make(map[string]*unit.UnitState),
	}
}

type UnitStatePublisher struct {
	reg  registry.Registry
	mach machine.Machine
	ttl  time.Duration

	mutex sync.RWMutex
	cache map[string]*unit.UnitState
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
				p.publishAll()
			}
		}
	}()

	machID := p.mach.State().ID

	for {
		select {
		case <-stop:
			return
		case bt := <-beatchan:
			if bt.State != nil {
				bt.State.MachineID = machID
			}

			p.updateCache(bt)
		}
	}
}

func (p *UnitStatePublisher) publishAll() {
	p.mutex.Lock()

	cache := make(map[string]*unit.UnitState)
	prev := p.cache
	for name, us := range p.cache {
		if us != nil {
			cache[name] = us
		}
	}
	p.cache = cache
	p.mutex.Unlock()

	for name, us := range prev {
		p.publishOne(name, us)
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

func (p *UnitStatePublisher) updateCache(update *unit.UnitStateHeartbeat) {
	p.mutex.Lock()

	last, ok := p.cache[update.Name]
	p.cache[update.Name] = update.State

	p.mutex.Unlock()

	// As an optimization, publish changes as they flow in
	if !ok || !reflect.DeepEqual(last, update.State) {
		p.publishOne(update.Name, update.State)
	}
}

func (p *UnitStatePublisher) Purge() {
	for name := range p.cache {
		p.publishOne(name, nil)
	}
}
