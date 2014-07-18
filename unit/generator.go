package unit

import (
	"time"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/pkg"
)

type UnitStateHeartbeat struct {
	Name  string
	State *UnitState
}

func NewUnitStateGenerator(mgr UnitManager) *UnitStateGenerator {
	return &UnitStateGenerator{mgr, pkg.NewThreadsafeSet(), nil}
}

type UnitStateGenerator struct {
	mgr UnitManager

	subscribed     pkg.Set
	lastSubscribed pkg.Set
}

// Run periodically calls Generate and sends received *UnitStateHeartbeat
// objects to the provided channel.
func (g *UnitStateGenerator) Run(receiver chan<- *UnitStateHeartbeat, stop chan bool) {
	tick := time.Tick(time.Second)
	for {
		select {
		case <-stop:
			return
		case <-tick:
			beatchan, err := g.Generate()
			if err != nil {
				log.Errorf("Failed fetching current unit states: %v", err)
				continue
			}

			for bt := range beatchan {
				receiver <- bt
			}
		}
	}
}

// Generate returns and fills a channel with *UnitStateHeartbeat objects. Objects will
// only be returned for units to which this generator is currently subscribed.
func (g *UnitStateGenerator) Generate() (<-chan *UnitStateHeartbeat, error) {
	subscribed := g.subscribed.Copy()
	reportable, err := g.mgr.GetUnitStates(subscribed)
	if err != nil {
		return nil, err
	}

	beatchan := make(chan *UnitStateHeartbeat)
	go func() {
		for name, us := range reportable {
			us := us
			beatchan <- &UnitStateHeartbeat{Name: name, State: us}
		}

		if g.lastSubscribed != nil {
			// For all units that were part of the subscription list
			// last time Generate ran, but are now not part of that
			// list, send nil-State heartbeats
			for _, name := range g.lastSubscribed.Sub(subscribed).Values() {
				beatchan <- &UnitStateHeartbeat{Name: name, State: nil}
			}
		}

		g.lastSubscribed = subscribed
		close(beatchan)
	}()

	return beatchan, nil
}

// Subscribe adds a unit to the internal state filter
func (g *UnitStateGenerator) Subscribe(name string) {
	g.subscribed.Add(name)
}

// Unsubscribe removes a unit from the internal state filter
func (g *UnitStateGenerator) Unsubscribe(name string) {
	g.subscribed.Remove(name)
}
