package engine

import (
	"fmt"
	"time"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
)

const (
	// time between triggering reconciliation routine
	reconcileInterval = 2 * time.Second

	// name of role that represents the lead engine in a cluster
	engineRoleName = "engine-leader"
	// time the role will be leased before the lease must be renewed
	engineRoleLeasePeriod = 10 * time.Second
)

type Engine struct {
	rec      Reconciler
	registry registry.Registry
	machine  machine.Machine

	lease registry.Lease
}

func New(reg registry.Registry, mach machine.Machine) *Engine {
	rec := &dumbReconciler{reg, mach}
	return &Engine{rec, reg, mach, nil}
}

func (e *Engine) Run(stop chan bool) {
	ticker := time.Tick(reconcileInterval)
	machID := e.machine.State().ID

	for {
		select {
		case <-stop:
			log.V(1).Info("Engine exiting due to stop signal")
			return
		case <-ticker:
			log.V(1).Info("Engine tick")

			e.lease = ensureLeader(e.lease, e.registry, machID)
			if e.lease == nil {
				continue
			}

			start := time.Now()
			e.rec.Reconcile()
			elapsed := time.Now().Sub(start)

			msg := fmt.Sprintf("Engine completed reconciliation in %s", elapsed)
			if elapsed > reconcileInterval {
				log.Warning(msg)
			} else {
				log.V(1).Info(msg)
			}
		}
	}
}

func (e *Engine) Purge() {
	if e.lease == nil {
		return
	}
	err := e.lease.Release()
	if err != nil {
		log.Errorf("Failed to release lease: %v", err)
	}
}

// ensureLeader will attempt to renew a non-nil Lease, falling back to
// acquiring a new Lease on the lead engine role.
func ensureLeader(prev registry.Lease, reg registry.Registry, machID string) (cur registry.Lease) {
	if prev != nil {
		err := prev.Renew(engineRoleLeasePeriod)
		if err == nil {
			cur = prev
			return
		} else {
			log.Errorf("Engine leadership could not be renewed: %v", err)
		}
	}

	var err error
	cur, err = reg.LeaseRole(engineRoleName, machID, engineRoleLeasePeriod)
	if err != nil {
		log.Errorf("Failed acquiring engine leadership: %v", err)
	} else if cur == nil {
		log.V(1).Infof("Unable to acquire engine leadership")
	} else {
		log.Infof("Acquired engine leadership")
	}

	return
}
