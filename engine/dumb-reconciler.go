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

func (r *dumbReconciler) Reconcile() {
	log.V(1).Infof("Polling Registry for actionable work")

	jobs, err := r.registry.Jobs()
	if err != nil {
		log.Errorf("Failed fetching Jobs from Registry: %v", err)
		return
	}

	offers, err := r.registry.UnresolvedJobOffers()
	if err != nil {
		log.Errorf("Failed fetching JobOffers from Registry: %v", err)
		return
	}

	machines, err := r.registry.Machines()
	if err != nil {
		log.Errorf("Failed fetching Machines from Registry: %v", err)
		return
	}

	oMap := make(map[string]*job.JobOffer, len(offers))
	for i := range offers {
		o := offers[i]
		oMap[o.Job.Name] = &o
	}

	mMap := make(map[string]*machine.MachineState, len(machines))
	for i := range machines {
		m := machines[i]
		mMap[m.ID] = &m
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

	// maybeScheduleJob attempts to schedule the given Job only if one or more
	// bids have been submitted
	maybeScheduleJob := func(jName string) error {
		bids, err := r.registry.Bids(oMap[jName])
		if err != nil {
			log.Errorf("Failed determining open JobBids for JobOffer(%s): %v", jName, err)
			return err
		}

		if len(bids) == 0 {
			log.Infof("No bids found for unresolved JobOffer(%s), unable to resolve", jName)
			return nil
		}

		choice := bids[0]

		err = r.registry.ScheduleJob(jName, choice.MachineID)
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

	// machinePresent determines if the referenced Machine appears to be a
	// current member of the cluster based on the local cache
	machinePresent := func(machID string) bool {
		_, ok := mMap[machID]
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
		if machinePresent(j.TargetMachineID) {
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
				r.offerJob(j)
			}
		}
	}

	for _, j := range activeNotScheduled {
		if !offerExists(j.Name) {
			log.Infof("Offering Job(%s) since target state %s and Job not scheduled", j.Name, j.TargetState)
			r.offerJob(j)
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

func (r *dumbReconciler) offerJob(j *job.Job) {
	offer := job.NewOfferFromJob(*j, nil)
	err := r.registry.CreateJobOffer(offer)
	if err != nil {
		log.Errorf("Failed publishing JobOffer(%s): %v", j.Name, err)
	} else {
		log.Infof("Published JobOffer(%s)", j.Name)
	}
}
