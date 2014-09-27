package engine

import (
	"fmt"
	"time"

	"github.com/coreos/fleet/log"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/pkg"
	"github.com/coreos/fleet/registry"
)

const (
	// name of lease that must be held by the lead engine in a cluster
	engineLeaseName = "engine-leader"
)

type Engine struct {
	rec       *Reconciler
	registry  registry.Registry
	cRegistry registry.ClusterRegistry
	rStream   pkg.EventStream
	machine   machine.Machine

	lease   registry.Lease
	trigger chan struct{}
}

func New(reg *registry.EtcdRegistry, rStream pkg.EventStream, mach machine.Machine) *Engine {
	rec := NewReconciler()
	return &Engine{
		rec:       rec,
		registry:  reg,
		cRegistry: reg,
		rStream:   rStream,
		machine:   mach,
		trigger:   make(chan struct{}),
	}
}

func (e *Engine) Run(ival time.Duration, stop chan bool) {
	leaseTTL := ival * 5
	machID := e.machine.State().ID

	reconcile := func() {
		e.lease = ensureLeader(e.lease, e.registry, machID, leaseTTL)
		if e.lease == nil {
			return
		}

		// abort is closed when reconciliation must stop prematurely, either
		// by a local timeout or the fleet server shutting down
		abort := make(chan struct{})

		// monitor is used to shut down the following goroutine
		monitor := make(chan struct{})

		go func() {
			select {
			case <-monitor:
				return
			case <-time.After(leaseTTL):
				close(abort)
			case <-stop:
				close(abort)
			}
		}()

		start := time.Now()
		e.rec.Reconcile(e, abort)
		close(monitor)
		elapsed := time.Now().Sub(start)

		msg := fmt.Sprintf("Engine completed reconciliation in %s", elapsed)
		if elapsed > ival {
			log.Warning(msg)
		} else {
			log.V(1).Info(msg)
		}
	}

	rec := pkg.NewPeriodicReconciler(ival, reconcile, e.rStream)
	rec.Run(stop)
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

// ensureLeader will attempt to renew the engine lease if it is already
// held. If it is not already held, it will attempt to acquire the lease.
func ensureLeader(prev registry.Lease, reg registry.Registry, machID string, ttl time.Duration) (cur registry.Lease) {
	if prev != nil {
		err := prev.Renew(ttl)
		if err == nil {
			log.V(1).Infof("Engine leadership renewed")
			cur = prev
			return
		} else {
			log.Errorf("Engine leadership lost, renewal failed: %v", err)
		}
	}

	var err error
	cur, err = reg.AcquireLease(engineLeaseName, machID, ttl)
	if err != nil {
		log.Errorf("Engine leadership acquisition failed: %v", err)
	} else if cur == nil {
		log.V(1).Infof("Unable to acquire engine leadership")
	} else {
		log.Infof("Engine leadership acquired")
	}

	return
}

func (e *Engine) Trigger() {
	e.trigger <- struct{}{}
}

func (e *Engine) clusterState() (*clusterState, error) {
	units, err := e.registry.Units()
	if err != nil {
		log.Errorf("Failed fetching Units from Registry: %v", err)
		return nil, err
	}

	sUnits, err := e.registry.Schedule()
	if err != nil {
		log.Errorf("Failed fetching schedule from Registry: %v", err)
		return nil, err
	}

	machines, err := e.registry.Machines()
	if err != nil {
		log.Errorf("Failed fetching Machines from Registry: %v", err)
		return nil, err
	}

	return newClusterState(units, sUnits, machines), nil
}

func (e *Engine) unscheduleUnit(name, machID string) (err error) {
	err = e.registry.UnscheduleUnit(name, machID)
	if err != nil {
		log.Errorf("Failed unscheduling Unit(%s) from Machine(%s): %v", name, machID, err)
	} else {
		log.Infof("Unscheduled Job(%s) from Machine(%s)", name, machID)
	}
	return
}

// attemptScheduleUnit tries to persist a scheduling decision in the
// Registry, returning true on success. If any communication with the
// Registry fails, false is returned.
func (e *Engine) attemptScheduleUnit(name, machID string) bool {
	err := e.registry.ScheduleUnit(name, machID)
	if err != nil {
		log.Errorf("Failed scheduling Unit(%s) to Machine(%s): %v", name, machID, err)
		return false
	}

	log.Infof("Scheduled Unit(%s) to Machine(%s)", name, machID)
	return true
}
