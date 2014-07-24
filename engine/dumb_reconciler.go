package engine

import (
	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"
)

type dumbReconciler struct{}

func (r *dumbReconciler) Reconcile(e *Engine) {
	log.V(1).Infof("Polling Registry for actionable work")

	clust, err := e.clusterState()
	if err != nil {
		log.Errorf("Failed getting current cluster state: %v", err)
		return
	}

	resolveJobOffer := func(jName string) {
		clust.forgetOffer(jName)
		e.resolveJobOffer(jName)
	}

	inactive := clust.inactiveJobs()
	needScheduling := clust.unscheduledLoadedJobs()
	loaded := clust.scheduledLoadedJobs()

	for _, j := range inactive {
		if j.Scheduled() {
			log.Infof("Unscheduling Job(%s) from Machine(%s) since target state is inactive %s", j.Name, j.TargetMachineID)
			err := e.unscheduleJob(j.Name, j.TargetMachineID)
			if err != nil {
				continue
			}
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
			err := e.unscheduleJob(j.Name, j.TargetMachineID)
			if err != nil {
				continue
			}

			if !clust.offerExists(j.Name) {
				log.Infof("Offering Job(%s) since target state %s and Job not scheduled", j.Name, j.TargetState)
				e.offerJob(j)
			}
		}
	}

	for _, j := range needScheduling {
		if !clust.offerExists(j.Name) {
			log.Infof("Offering Job(%s) since target state %s and Job not scheduled", j.Name, j.TargetState)
			e.offerJob(j)
			continue
		}

		log.V(1).Infof("Attempting to schedule Job(%s) since target state %s and Job not scheduled", j.Name, j.TargetState)

		if e.attemptScheduleJob(j.Name) {
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
