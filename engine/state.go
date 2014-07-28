package engine

import (
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/pkg"
)

type clusterState struct {
	jobs     []job.Job
	offers   pkg.Set
	machines pkg.Set
}

func newClusterState(jobs []job.Job, unresolved []job.JobOffer, machines []machine.MachineState) *clusterState {
	oSet := pkg.NewUnsafeSet()
	for _, offer := range unresolved {
		oSet.Add(offer.Job.Name)
	}

	mSet := pkg.NewUnsafeSet()
	for _, m := range machines {
		mSet.Add(m.ID)
	}

	return &clusterState{
		jobs:     jobs,
		offers:   oSet,
		machines: mSet,
	}
}

// inactiveJobs returns a collection of Jobs that have a target
// state of "inactive"
func (cs *clusterState) inactiveJobs() []*job.Job {
	jobs := make([]*job.Job, 0)
	for i := range cs.jobs {
		j := cs.jobs[i]
		if j.TargetState == job.JobStateInactive {
			jobs = append(jobs, &j)
		}
	}
	return jobs
}

// unscheduledLoadedJobs returns a collection of Jobs that have a
// target state other than "inactive", but have not been scheduled
func (cs *clusterState) unscheduledLoadedJobs() []*job.Job {
	jobs := make([]*job.Job, 0)
	for i := range cs.jobs {
		j := cs.jobs[i]
		if j.TargetState != job.JobStateInactive && !j.Scheduled() {
			jobs = append(jobs, &j)
		}
	}
	return jobs
}

// scheduledLoadedJobs returns a collection of Jobs that have a
// target state other than "inactive" and been scheduled
func (cs *clusterState) scheduledLoadedJobs() []*job.Job {
	jobs := make([]*job.Job, 0)
	for i := range cs.jobs {
		j := cs.jobs[i]
		if j.TargetState != job.JobStateInactive && j.Scheduled() {
			jobs = append(jobs, &j)
		}
	}
	return jobs
}

func (cs *clusterState) unresolvedOffers() (offers []string) {
	offers = make([]string, cs.offers.Length())
	for i, offer := range cs.offers.Values() {
		offer := offer
		offers[i] = offer
	}
	return
}

// forgetOffer removes a JobOffer from the clusterState
func (cs *clusterState) forgetOffer(jName string) {
	cs.offers.Remove(jName)
}

// offerExists returns true if the referenced JobOffer appears
// in the clusterState's collection of unresolved offers
func (cs *clusterState) offerExists(jName string) bool {
	return cs.offers.Contains(jName)
}

func (cs *clusterState) machineExists(machID string) bool {
	return cs.machines.Contains(machID)
}
