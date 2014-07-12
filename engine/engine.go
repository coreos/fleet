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

// Reconcile attempts to advance the state of Jobs towards their desired states
// wherever discrepancies are identified.
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

	machines, err := e.registry.Machines()
	if err != nil {
		log.Errorf("Failed fetching Machines from Registry: %v", err)
		return
	}

	// Initialize the cached view of the cluster, tracking all known machines
	clust := newCluster()
	for _, m := range machines {
		m := m
		clust.TrackMachine(&m)
	}

	// Jobs will be sorted into three categories:
	// 1. Jobs where TargetState is inactive
	// 2. Jobs where TargetState is active, and the Job has been scheduled
	// 3. Jobs where TargetState is active, and the Job has not been scheduled
	inactive := make([]*job.Job, 0)
	activeScheduled := make([]*job.Job, 0)
	activeNotScheduled := make([]*job.Job, 0)

	for _, j := range jobs {
		j := j
		clust.AddJob(&j)
		if j.TargetState == job.JobStateInactive {
			inactive = append(inactive, &j)
		} else if j.Scheduled() {
			activeScheduled = append(activeScheduled, &j)
		} else {
			activeNotScheduled = append(activeNotScheduled, &j)
		}
	}

	// Unschedule inactive jobs
	for _, j := range inactive {
		if j.Scheduled() {
			log.Infof("Unscheduling Job(%s) from Machine(%s) since target state is inactive %s", j.Name, j.TargetMachineID)
			e.unscheduleJob(j)
		}
	}

	// Unschedule active jobs with dead machines
	for _, j := range activeScheduled {
		if !clust.MachinePresent(j.TargetMachineID) {
			log.Infof("Unscheduling Job(%s) since target Machine(%s) seems to have gone away", j.Name, j.TargetMachineID)
			if err := e.unscheduleJob(j); err == nil {
				// Add this job for re-scheduling
				activeNotScheduled = append(activeNotScheduled, j)
			}
		}
	}

	// Schedule active-but-unscheduled jobs
	for _, j := range activeNotScheduled {
		machIDs := clust.Candidates(j)
		if len(machIDs) == 0 {
			log.Infof("Unable to schedule Job(%s): no machines meet requirements", j.Name)
		} else {
			mID := machIDs[0]
			if err := e.scheduleJob(j.Name, mID); err != nil {
				// This Job was successfully scheduled,
				// so update the cluster view for subsequent
				// scheduling decisions
				clust.AddJob(j)
				clust.ScheduleJob(mID, j)
			}
		}
	}
}

// unscheduleJob clears the current target of the provided Job from
// the Registry
func (e *Engine) unscheduleJob(j *job.Job) (err error) {
	err = e.registry.ClearJobTarget(j.Name, j.TargetMachineID)
	if err != nil {
		log.Errorf("Failed clearing target Machine(%s) of Job(%s): %v", j.TargetMachineID, j.Name, err)
	} else {
		log.Infof("Unscheduled Job(%s) from Machine(%s)", j.Name, j.TargetMachineID)
	}

	return
}

// scheduleJob schedules a Job directly to a target machine by setting
// the target in the Registry
func (e *Engine) scheduleJob(jName, machID string) (err error) {
	err = e.registry.ScheduleJob(jName, machID)
	if err != nil {
		log.Errorf("Failed scheduling Job(%s) to Machine(%s): %v", jName, machID, err)
	} else {
		log.Infof("Scheduled Job(%s) to Machine(%s)", jName, machID)
	}
	return
}
