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

	// version at which the current engine code operates
	engineVersion = 1
)

type Engine struct {
	rec       *Reconciler
	registry  registry.Registry
	cRegistry registry.ClusterRegistry
	lRegistry registry.LeaseRegistry
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
		lRegistry: reg,
		rStream:   rStream,
		machine:   mach,
		trigger:   make(chan struct{}),
	}
}

func (e *Engine) Run(ival time.Duration, stop chan bool) {
	leaseTTL := ival * 5
	machID := e.machine.State().ID

	reconcile := func() {
		if !ensureEngineVersionMatch(e.cRegistry, engineVersion) {
			return
		}

		var l registry.Lease
		if isLeader(e.lease, machID) {
			l = renewLeadership(e.lease, leaseTTL)
		} else {
			l = acquireLeadership(e.lRegistry, machID, engineVersion, leaseTTL)
		}

		// log all leadership changes
		if l != nil && e.lease == nil && l.MachineID() != machID {
			log.Infof("Engine leader is %s", l.MachineID())
		} else if l != nil && e.lease != nil && l.MachineID() != e.lease.MachineID() {
			log.Infof("Engine leadership changed from %s to %s", e.lease.MachineID(), l.MachineID())
		}

		e.lease = l

		if !isLeader(e.lease, machID) {
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
	// only purge the lease if we are the leader
	if !isLeader(e.lease, e.machine.State().ID) {
		return
	}
	err := e.lease.Release()
	if err != nil {
		log.Errorf("Failed to release lease: %v", err)
	}
}

func isLeader(l registry.Lease, machID string) bool {
	if l == nil {
		return false
	}
	if l.MachineID() != machID {
		return false
	}
	return true
}

func ensureEngineVersionMatch(cReg registry.ClusterRegistry, expect int) bool {
	v, err := cReg.EngineVersion()
	if err != nil {
		log.Errorf("Unable to determine cluster engine version")
		return false
	}

	if v < expect {
		err = cReg.UpdateEngineVersion(v, expect)
		if err != nil {
			log.Errorf("Failed updating cluster engine version from %d to %d: %v", v, expect, err)
			return false
		}
		log.Infof("Updated cluster engine version from %d to %d", v, expect)
	} else if v > expect {
		log.V(1).Infof("Cluster engine version higher than local engine version (%d > %d), unable to participate", v, expect)
		return false
	}

	return true
}

func acquireLeadership(lReg registry.LeaseRegistry, machID string, ver int, ttl time.Duration) registry.Lease {
	existing, err := lReg.GetLease(engineLeaseName)
	if err != nil {
		log.Errorf("Unable to determine current lessee: %v", err)
		return nil
	}

	var l registry.Lease
	if existing == nil {
		l, err = lReg.AcquireLease(engineLeaseName, machID, ver, ttl)
		if err != nil {
			log.Errorf("Engine leadership acquisition failed: %v", err)
			return nil
		} else if l == nil {
			log.V(1).Infof("Unable to acquire engine leadership")
			return nil
		}
		log.Infof("Engine leadership acquired")
		return l
	}

	if existing.Version() >= ver {
		log.V(1).Infof("Lease already held by Machine(%s) operating at acceptable version %d", existing.MachineID(), existing.Version())
		return existing
	}

	rem := existing.TimeRemaining()
	l, err = lReg.StealLease(engineLeaseName, machID, ver, ttl+rem, existing.Index())
	if err != nil {
		log.Errorf("Engine leadership steal failed: %v", err)
		return nil
	} else if l == nil {
		log.V(1).Infof("Unable to steal engine leadership")
		return nil
	}

	log.Infof("Stole engine leadership from Machine(%s)", existing.MachineID())

	if rem > 0 {
		log.Infof("Waiting %v for previous lease to expire before continuing reconciliation", rem)
		<-time.After(rem)
	}

	return l
}

func renewLeadership(l registry.Lease, ttl time.Duration) registry.Lease {
	err := l.Renew(ttl)
	if err != nil {
		log.Errorf("Engine leadership lost, renewal failed: %v", err)
		return nil
	}

	log.V(1).Infof("Engine leadership renewed")
	return l
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
