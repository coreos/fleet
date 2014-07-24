package engine

import (
	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
)

type dumbReconciler struct {
	registry registry.Registry
	machine  machine.Machine
}

func (r *dumbReconciler) Reconcile(e *Engine) {
	log.V(1).Infof("Polling Registry for actionable work")

	clust, err := e.clusterState()
	if err != nil {
		log.Errorf("Failed getting current cluster state: %v", err)
		return
	}

	resolveJobOffer := func(jName string) {
		clust.forgetOffer(jName)

		err := r.registry.ResolveJobOffer(jName)
		if err != nil {
			log.Errorf("Failed resolving JobOffer(%s): %v", jName, err)
		} else {
			log.Infof("Resolved JobOffer(%s)", jName)
		}
	}

	// unscheduleJob clears the current target of the provided Job from
	// the Registry
	unscheduleJob := func(j *job.Job) (err error) {
		err = r.registry.ClearJobTarget(j.Name, j.TargetMachineID)
		if err != nil {
			log.Errorf("Failed clearing target Machine(%s) of Job(%s): %v", j.TargetMachineID, j.Name, err)
		} else {
			log.Infof("Unscheduled Job(%s) from Machine(%s)", j.Name, j.TargetMachineID)
		}

		return
	}

	// scheduleJob attempts to schedule the given Job only if one or more
	// bids have been submitted
	scheduleJob := func(jName string) bool {
		bids, err := r.registry.Bids(jName)
		if err != nil {
			log.Errorf("Failed determining open JobBids for JobOffer(%s): %v", jName, err)
			return false
		}

		if bids.Length() == 0 {
			log.V(1).Infof("No bids found for unresolved JobOffer(%s), unable to resolve", jName)
			return false
		}

		choice := bids.Values()[0]

		err = r.registry.ScheduleJob(jName, choice)
		if err != nil {
			log.Errorf("Failed scheduling Job(%s) to Machine(%s): %v", jName, choice, err)
			return false
		}

		log.Infof("Scheduled Job(%s) to Machine(%s)", jName, choice)
		return true
	}

	inactive := clust.inactiveJobs()
	needScheduling := clust.unscheduledLoadedJobs()
	loaded := clust.scheduledLoadedJobs()

	for _, j := range inactive {
		if j.Scheduled() {
			log.Infof("Unscheduling Job(%s) from Machine(%s) since target state is inactive %s", j.Name, j.TargetMachineID)
			unscheduleJob(j)
		}

		if clust.offerExists(j.Name) {
			log.Infof("Resolving extraneous JobOffer(%s) since target state is inactive", j.Name)
			resolveJobOffer(j.Name)
		}
	}

	for _, j := range loaded {
		if clust.machineExists(j.TargetMachineID) {
			if clust.offerExists(j.Name) {
				log.Infof("Resolving extraneous offer since Job(%s) is already scheduled", j.Name)
				resolveJobOffer(j.Name)
			}
		} else {
			log.Infof("Unscheduling Job(%s) since target Machine(%s) seems to have gone away", j.Name, j.TargetMachineID)
			err := unscheduleJob(j)
			if err != nil {
				continue
			}

			if !clust.offerExists(j.Name) {
				log.Infof("Offering Job(%s) since target state %s and Job not scheduled", j.Name, j.TargetState)
				r.offerJob(j)
			}
		}
	}

	for _, j := range needScheduling {
		if !clust.offerExists(j.Name) {
			log.Infof("Offering Job(%s) since target state %s and Job not scheduled", j.Name, j.TargetState)
			r.offerJob(j)
			continue
		}

		log.V(1).Infof("Attempting to schedule Job(%s) since target state %s and Job not scheduled", j.Name, j.TargetState)

		if !scheduleJob(j.Name) {
			clust.forgetOffer(j.Name)
			continue
		}

		resolveJobOffer(j.Name)
	}

	// Deal with remaining JobOffers that do not have a corresponding Job
	for _, jName := range clust.unresolvedOffers() {
		log.Infof("Destroying JobOffer(%s) since corresponding Job does not exist", jName)
		resolveJobOffer(jName)
	}
}

func (r *dumbReconciler) offerJob(j *job.Job) {
	offer := job.NewOfferFromJob(*j)
	err := r.registry.CreateJobOffer(offer)
	if err != nil {
		log.Errorf("Failed publishing JobOffer(%s): %v", j.Name, err)
	} else {
		log.Infof("Published JobOffer(%s)", j.Name)
	}
}
