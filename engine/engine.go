package engine

import (
	"fmt"
	"time"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
)

const (
	// name of role that represents the lead engine in a cluster
	engineRoleName = "engine-leader"
)

type Engine struct {
	rec      Reconciler
	registry registry.Registry
	machine  machine.Machine

	lease   registry.Lease
	trigger chan struct{}
}

func New(reg registry.Registry, mach machine.Machine) *Engine {
	rec := &dumbReconciler{}
	return &Engine{rec, reg, mach, nil, make(chan struct{})}
}

func (e *Engine) Run(ival time.Duration, stop chan bool) {
	leaseTTL := ival * 5
	ticker := time.Tick(ival)
	machID := e.machine.State().ID

	reconcile := func() {
		done := make(chan struct{})
		defer func() { close(done) }()
		// While the reconciliation is running, flush the trigger channel in the background
		go func() {
			for {
				select {
				case <-done:
					return
				default:
					select {
					case <-e.trigger:
					case <-done:
						return
					}
				}
			}
		}()

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

	for {
		select {
		case <-stop:
			log.V(1).Info("Engine exiting due to stop signal")
			return
		case <-ticker:
			log.V(1).Info("Engine tick")
			reconcile()
		case <-e.trigger:
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

	offers, err := e.registry.UnresolvedJobOffers()
	if err != nil {
		log.Errorf("Failed fetching JobOffers from Registry: %v", err)
		return nil, err
	}

	machines, err := e.registry.Machines()
	if err != nil {
		log.Errorf("Failed fetching Machines from Registry: %v", err)
		return nil, err
	}

	return newClusterState(jobs, offers, machines), nil
}

func (e *Engine) resolveJobOffer(jName string) (err error) {
	err = e.registry.ResolveJobOffer(jName)
	if err != nil {
		log.Errorf("Failed resolving JobOffer(%s): %v", jName, err)
	} else {
		log.Infof("Resolved JobOffer(%s)", jName)
	}
	return
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

// attemptScheduleJob accepts a bid for the given Job and persists the
// decision to the registry, returning true on success. If no bids exist or
// if any communication with the Registry fails, false is returned.
func (e *Engine) attemptScheduleJob(jName string) bool {
	bids, err := e.registry.Bids(jName)
	if err != nil {
		log.Errorf("Failed determining open JobBids for JobOffer(%s): %v", jName, err)
		return false
	}

	if bids.Length() == 0 {
		log.V(1).Infof("No bids found for unresolved JobOffer(%s), unable to resolve", jName)
		return false
	}

	choice := bids.Values()[0]

	err = e.registry.ScheduleJob(jName, choice)
	if err != nil {
		log.Errorf("Failed scheduling Job(%s) to Machine(%s): %v", jName, choice, err)
		return false
	}

	log.Infof("Scheduled Job(%s) to Machine(%s)", jName, choice)
	return true
}

func (e *Engine) offerJob(j *job.Job) (err error) {
	offer := job.NewOfferFromJob(*j)
	err = e.registry.CreateJobOffer(offer)
	if err != nil {
		log.Errorf("Failed publishing JobOffer(%s): %v", j.Name, err)
	} else {
		log.Infof("Published JobOffer(%s)", j.Name)
	}
	return
}
