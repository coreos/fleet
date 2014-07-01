package engine

import (
	"errors"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
)

type Engine struct {
	registry registry.Registry
	machine  machine.Machine
	// keeps a picture of the load in the cluster for more intelligent scheduling
	clust *cluster
}

func New(reg registry.Registry, mach machine.Machine) *Engine {
	return &Engine{reg, mach, newCluster()}
}

// CheckForWork attempts to rectify the current state of all Jobs in the cluster
// with their target states wherever discrepancies are identified.
func (e *Engine) CheckForWork() {
	log.Infof("Polling etcd for actionable Jobs")

	for _, jo := range e.registry.UnresolvedJobOffers() {
		bids, err := e.registry.Bids(&jo)
		if err != nil {
			log.Errorf("Failed determining open JobBids for JobOffer(%s): %v", jo.Job.Name, err)
			continue
		}
		if len(bids) == 0 {
			log.V(1).Infof("No bids found for open JobOffer(%s), ignoring", jo.Job.Name)
			continue
		}

		log.Infof("Resolving JobOffer(%s), scheduling to Machine(%s)", bids[0].JobName, bids[0].MachineID)
		if e.ResolveJobOffer(bids[0].JobName, bids[0].MachineID); err != nil {
			log.Infof("Failed scheduling Job(%s) to Machine(%s)", bids[0].JobName, bids[0].MachineID)
		} else {
			log.Infof("Scheduled Job(%s) to Machine(%s)", bids[0].JobName, bids[0].MachineID)
		}
	}

	jobs, _ := e.registry.Jobs()
	for _, j := range jobs {
		if j.TargetState == nil || j.State == nil || *j.TargetState == *j.State {
			continue
		}

		if *j.State == job.JobStateInactive {
			log.Infof("Offering Job(%s)", j.Name)
			e.OfferJob(j)
		} else if *j.TargetState == job.JobStateInactive {
			log.Infof("Unscheduling Job(%s)", j.Name)
			e.registry.ClearJobTarget(j.Name, j.TargetMachineID)
		}
	}
}

func (e *Engine) OfferJob(j job.Job) error {
	log.V(1).Infof("Attempting to lock Job(%s)", j.Name)

	mutex := e.registry.LockJob(j.Name, e.machine.State().ID)
	if mutex == nil {
		log.V(1).Infof("Could not lock Job(%s)", j.Name)
		return errors.New("could not lock Job")
	}
	defer mutex.Unlock()

	log.V(1).Infof("Claimed Job(%s)", j.Name)

	machineIDs, err := e.partitionCluster(&j)
	if err != nil {
		log.Errorf("failed partitioning cluster for Job(%s): %v", j.Name, err)
		return err
	}

	offer := job.NewOfferFromJob(j, machineIDs)

	err = e.registry.CreateJobOffer(offer)
	if err == nil {
		log.Infof("Published JobOffer(%s)", offer.Job.Name)
	}

	return err
}

func (e *Engine) ResolveJobOffer(jobName string, machID string) error {
	log.V(1).Infof("Attempting to lock JobOffer(%s)", jobName)
	mutex := e.registry.LockJobOffer(jobName, e.machine.State().ID)

	if mutex == nil {
		log.V(1).Infof("Could not lock JobOffer(%s)", jobName)
		return errors.New("could not lock JobOffer")
	}
	defer mutex.Unlock()

	log.V(1).Infof("Claimed JobOffer(%s)", jobName)

	err := e.registry.ResolveJobOffer(jobName)
	if err != nil {
		log.Errorf("Failed resolving JobOffer(%s): %v", jobName, err)
		return err
	}

	err = e.registry.ScheduleJob(jobName, machID)
	if err != nil {
		log.Errorf("Failed scheduling Job(%s): %v", jobName, err)
		return err
	}

	log.Infof("Scheduled Job(%s) to Machine(%s)", jobName, machID)
	return nil
}
