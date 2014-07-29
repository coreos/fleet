package engine

import (
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/pkg"
)

type clusterState struct {
	jobs     []job.Job
	offers   map[string]pkg.Set
	machines pkg.Set
}

func newClusterState(jobs []job.Job, offers map[string]pkg.Set, machines []machine.MachineState) *clusterState {
	mSet := pkg.NewUnsafeSet()
	for _, m := range machines {
		mSet.Add(m.ID)
	}

	return &clusterState{
		jobs:     jobs,
		offers:   offers,
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

// forgetOffer removes a JobOffer from the clusterState
func (cs *clusterState) forgetOffer(jName string) {
	delete(cs.offers, jName)
}

// offerExists returns true if the referenced JobOffer appears
// in the clusterState's collection of unresolved offers
func (cs *clusterState) offerExists(jName string) bool {
	_, ok := cs.offers[jName]
	return ok
}

func (cs *clusterState) machineExists(machID string) bool {
	return cs.machines.Contains(machID)
}
