package engine

import (
	"fmt"
	"time"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
)

const (
	// name of role that represents the lead engine in a cluster
	engineRoleName = "engine-leader"
)

type Engine struct {
	rec      *Reconciler
	registry registry.Registry
	rStream  registry.EventStream
	machine  machine.Machine

	lease   registry.Lease
	trigger chan struct{}
}

func New(reg registry.Registry, rStream registry.EventStream, mach machine.Machine) *Engine {
	rec := NewReconciler()
	return &Engine{rec, reg, rStream, mach, nil, make(chan struct{})}
}

func (e *Engine) Run(ival time.Duration, stop chan bool) {
	leaseTTL := ival * 5
	ticker := time.Tick(ival)
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

	trigger := make(chan struct{})
	go func() {
		abort := make(chan struct{})
		select {
		case <-stop:
			close(abort)
		case <-e.rStream.Next(abort):
			trigger <- struct{}{}
		}
	}()

	for {
		select {
		case <-stop:
			log.V(1).Info("Engine exiting due to stop signal")
			return
		case <-ticker:
			log.V(1).Info("Engine tick")
			reconcile()
		case <-trigger:
			log.V(1).Info("Engine reconcilation triggered by job state change")
			reconcile()
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
	cur, err = reg.LeaseRole(engineRoleName, machID, ttl)
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
	jobs, err := e.registry.Jobs()
	if err != nil {
		log.Errorf("Failed fetching Jobs from Registry: %v", err)
		return nil, err
	}

	machines, err := e.registry.Machines()
	if err != nil {
		log.Errorf("Failed fetching Machines from Registry: %v", err)
		return nil, err
	}

	return newClusterState(jobs, machines), nil
}

func (e *Engine) unscheduleJob(jName, machID string) (err error) {
	err = e.registry.ClearJobTarget(jName, machID)
	if err != nil {
		log.Errorf("Failed clearing target Machine(%s) of Job(%s): %v", machID, jName, err)
	} else {
		log.Infof("Unscheduled Job(%s) from Machine(%s)", jName, machID)
	}
	return
}

// attemptScheduleJob tries to persist a scheduling decision in the
// Registry, returning true on success. If any communication with the
// Registry fails, false is returned.
func (e *Engine) attemptScheduleJob(jName, machID string) bool {
	err := e.registry.ScheduleJob(jName, machID)
	if err != nil {
		log.Errorf("Failed scheduling Job(%s) to Machine(%s): %v", jName, machID, err)
		return false
	}

	log.Infof("Scheduled Job(%s) to Machine(%s)", jName, machID)
	return true
}
