package agent

import (
	"reflect"
	"sync"
	"time"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/unit"
)

func NewUnitStatePublisher(mgr unit.UnitManager, reg registry.Registry, mach machine.Machine) *UnitStatePublisher {
	return &UnitStatePublisher{
		mgr:   mgr,
		reg:   reg,
		mach:  mach,
		mutex: sync.RWMutex{},
		cache: make(map[string]*unit.UnitState),
	}
}

type UnitStatePublisher struct {
	mgr  unit.UnitManager
	reg  registry.Registry
	mach machine.Machine

	mutex sync.RWMutex
	cache map[string]*unit.UnitState
}

// Run caches all of the heartbeat objects from the provided channel, publishing
// them to the Registry every 5s. Heartbeat objects are also published as they
// are received on the channel.
func (p *UnitStatePublisher) Run(beatchan <-chan *unit.UnitStateHeartbeat, stop chan bool) {
	go func() {
		tick := time.Tick(5 * time.Second)
		for {
			select {
			case <-stop:
				return
			case <-tick:
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

			p.addToCache(bt)
		}
	}
}

func (p *UnitStatePublisher) publishAll() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	cache := make(map[string]*unit.UnitState)
	for name, us := range p.cache {
		p.publishOne(name, us)
		if us != nil {
			cache[name] = us
		}
	}

	p.cache = cache
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
			p.reg.SaveUnitState(name, us)
		}
	}
}

func (p *UnitStatePublisher) addToCache(update *unit.UnitStateHeartbeat) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	last := p.cache[update.Name]
	p.cache[update.Name] = update.State

	// As an optimization, publish changes as they flow in
	if !reflect.DeepEqual(last, update.State) {
		go p.publishOne(update.Name, update.State)
	}
}
