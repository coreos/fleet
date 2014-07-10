package engine

import (
	"time"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/job"
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
	registry registry.Registry
	machine  machine.Machine
	lease    registry.Lease
}

func New(reg registry.Registry, mach machine.Machine) *Engine {
	return &Engine{reg, mach, nil}
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

			e.Reconcile()
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

// Reconcile attempts to advance the state of Jobs and JobOffers
// towards their desired states wherever discrepancies are identified.
func (e *Engine) Reconcile() {
	log.V(1).Infof("Polling Registry for actionable work")

	start := time.Now()
	defer func() {
		log.Infof("Engine completed reconciliation in %s", time.Now().Sub(start))
	}()

	jobs, err := e.registry.Jobs()
	if err != nil {
		log.Errorf("Failed fetching Jobs from Registry: %v", err)
		return
	}

	offers, err := e.registry.UnresolvedJobOffers()
	if err != nil {
		log.Errorf("Failed fetching JobOffers from Registry: %v", err)
		return
	}

	machines, err := e.registry.Machines()
	if err != nil {
		log.Errorf("Failed fetching Machines from Registry: %v", err)
		return
	}

	oMap := make(map[string]*job.JobOffer, len(offers))
	for i := range offers {
		o := offers[i]
		oMap[o.Job.Name] = &o
	}

	// Initialize the cached view of the cluster, tracking all known machines
	clust := newCluster()
	for _, m := range machines {
		m := m
		clust.trackMachine(&m)
	}

	// Jobs will be sorted into three categories:
	// 1. Jobs where TargetState is inactive
	inactive := make([]*job.Job, 0)
	// 2. Jobs where TargetState is active, and the Job has been scheduled
	activeScheduled := make([]*job.Job, 0)
	// 3. Jobs where TargetState is active, and the Job has not been scheduled
	activeNotScheduled := make([]*job.Job, 0)

	for i := range jobs {
		j := jobs[i]
		if j.TargetState == job.JobStateInactive {
			inactive = append(inactive, &j)
		} else if j.Scheduled() {
			activeScheduled = append(activeScheduled, &j)
		} else {
			activeNotScheduled = append(activeNotScheduled, &j)
		}
	}

	// resolveJobOffer removes the referenced Job's corresponding
	// JobOffer from the local cache before marking it as resolved
	// in the Registry
	resolveJobOffer := func(jName string) {
		delete(oMap, jName)

		err := e.registry.ResolveJobOffer(jName)
		if err != nil {
			log.Errorf("Failed resolving JobOffer(%s): %v", jName, err)
		} else {
			log.Infof("Resolved JobOffer(%s)", jName)
		}
	}

	// unscheduleJob clears the current target of the provided Job from
	// the Registry
	unscheduleJob := func(j *job.Job) (err error) {
		err = e.registry.ClearJobTarget(j.Name, j.TargetMachineID)
		if err != nil {
			log.Errorf("Failed clearing target Machine(%s) of Job(%s): %v", j.TargetMachineID, j.Name, err)
		} else {
			log.Infof("Unscheduled Job(%s) from Machine(%s)", j.Name, j.TargetMachineID)
		}

		return
	}

	// maybeScheduleJob attempts to schedule the given Job only if one or more
	// bids have been submitted
	maybeScheduleJob := func(jName string) error {
		bids, err := e.registry.Bids(oMap[jName])
		if err != nil {
			log.Errorf("Failed determining open JobBids for JobOffer(%s): %v", jName, err)
			return err
		}

		if len(bids) == 0 {
			log.Infof("No bids found for unresolved JobOffer(%s), unable to resolve", jName)
			return nil
		}

		choice := bids[0]

		err = e.registry.ScheduleJob(jName, choice.MachineID)
		if err != nil {
			log.Errorf("Failed scheduling Job(%s) to Machine(%s): %v", choice.JobName, choice.MachineID, err)
		} else {
			log.Infof("Scheduled Job(%s) to Machine(%s)", jName, choice.MachineID)
		}

		return err
	}

	// offerExists returns true if the referenced Job has a corresponding
	// offer in the local JobOffer cache
	offerExists := func(jobName string) bool {
		_, ok := oMap[jobName]
		return ok
	}

	for _, j := range inactive {
		if j.Scheduled() {
			log.Infof("Unscheduling Job(%s) from Machine(%s) since target state is inactive %s", j.Name, j.TargetMachineID)
			unscheduleJob(j)
		}

		if offerExists(j.Name) {
			log.Infof("Resolving extraneous JobOffer(%s) since target state is inactive", j.Name)
			resolveJobOffer(j.Name)
		}
	}

	for _, j := range activeScheduled {
		if clust.machinePresent(j.TargetMachineID) {
			if offerExists(j.Name) {
				log.Infof("Resolving extraneous JobOffer(%s) since Job is already scheduled", j.Name)
				resolveJobOffer(j.Name)
			}
		} else {
			log.Infof("Unscheduling Job(%s) since target Machine(%s) seems to have gone away", j.Name, j.TargetMachineID)
			err := unscheduleJob(j)
			if err != nil {
				continue
			}

			if !offerExists(j.Name) {
				log.Infof("Offering Job(%s) since target state %s and Job not scheduled", j.Name, j.TargetState)
				e.offerJob(clust, j)
			}
		}
	}

	for _, j := range activeNotScheduled {
		if !offerExists(j.Name) {
			log.Infof("Offering Job(%s) since target state %s and Job not scheduled", j.Name, j.TargetState)
			e.offerJob(clust, j)
			continue
		}

		log.Infof("Attempting to schedule Job(%s) since target state %s and Job not scheduled", j.Name, j.TargetState)

		err := maybeScheduleJob(j.Name)
		if err != nil {
			continue
		}

		resolveJobOffer(j.Name)
	}

	// Deal with remaining JobOffers that do not have a corresponding Job
	for jName, _ := range oMap {
		log.Infof("Removing extraneous JobOffer(%s) since corresponding Job does not exist", jName)
		resolveJobOffer(jName)
	}
}

func (e *Engine) offerJob(clust *cluster, j *job.Job) {
	machineIDs := clust.partition(j)
	offer := job.NewOfferFromJob(*j, machineIDs)
	err := e.registry.CreateJobOffer(offer)
	if err != nil {
		log.Errorf("Failed publishing JobOffer(%s): %v", j.Name, err)
	} else {
		log.Infof("Published JobOffer(%s)", j.Name)
	}
}
