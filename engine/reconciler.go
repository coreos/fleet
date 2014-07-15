package engine

import (
	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
)

// Reconciler attempts to advance the state of Jobs and JobOffers
// towards their desired states wherever discrepancies are identified.
type Reconciler interface {
	Reconcile()
}

type wipReconciler struct {
	reg     registry.Registry
	machine machine.Machine
}

func (r *wipReconciler) Reconcile() {
	log.V(1).Infof("Polling Registry for actionable work")

	jobs, err := r.reg.Jobs()
	if err != nil {
		log.Errorf("Failed fetching Jobs from Registry: %v", err)
		return
	}

	machines, err := r.reg.Machines()
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
		// TODO(jonboulle): consider only adding Active Jobs to the cluster, so we can optimistically make decisions about machines where Jobs will be unscheduled after this Reconcile
		clust.TrackJob(&j)
		if j.TargetState == job.JobStateInactive {
			inactive = append(inactive, &j)
		} else if j.Scheduled() {
			activeScheduled = append(activeScheduled, &j)
		} else {
			activeNotScheduled = append(activeNotScheduled, &j)
		}
	}

	// unscheduleJob clears the current target of the provided Job from
	// the Registry
	unscheduleJob := func(j *job.Job) (err error) {
		err = r.reg.ClearJobTarget(j.Name, j.TargetMachineID)
		if err != nil {
			log.Errorf("Failed clearing target Machine(%s) of Job(%s): %v", j.TargetMachineID, j.Name, err)
		} else {
			log.Infof("Unscheduled Job(%s) from Machine(%s)", j.Name, j.TargetMachineID)
		}

		return
	}

	// scheduleJob schedules a Job directly to a target machine by setting
	// the target in the Registry
	scheduleJob := func(jName, machID string) (err error) {
		err = r.reg.ScheduleJob(jName, machID)
		if err != nil {
			log.Errorf("Failed scheduling Job(%s) to Machine(%s): %v", jName, machID, err)
		} else {
			log.Infof("Scheduled Job(%s) to Machine(%s)", jName, machID)
		}
		return
	}

	// Unschedule inactive jobs
	for _, j := range inactive {
		if j.Scheduled() {
			log.Infof("Unscheduling Job(%s) from Machine(%s) since target state is inactive %s", j.Name, j.TargetMachineID)
			unscheduleJob(j)
		}
	}

	// Unschedule active jobs with dead machines
	for _, j := range activeScheduled {
		if !clust.MachinePresent(j.TargetMachineID) {
			log.Infof("Unscheduling Job(%s) since target Machine(%s) seems to have gone away", j.Name, j.TargetMachineID)
			if err := unscheduleJob(j); err == nil {
				// Add this job for re-scheduling
				activeNotScheduled = append(activeNotScheduled, j)
			}
		}
	}

	// Schedule active-but-unscheduled jobs
	decisions := clust.Decisions(activeNotScheduled)
	if err != nil {
		log.Errorf("Error determining cluster scheduling: %v", err)
		return
	}
	for _, dec := range decisions {
		if dec.Reason != nil {
			log.Infof("Unable to schedule Job(%s): %v", dec.Name, dec.Reason)
		} else {
			scheduleJob(dec.Name, dec.Machine)
		}
	}
}
