package engine

import (
	"github.com/coreos/fleet/agent"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
)

type clusterState struct {
	jobs   []job.Job
	agents map[string]*agent.AgentState
}

func newClusterState(jobs []job.Job, machines []machine.MachineState) *clusterState {
	agents := make(map[string]*agent.AgentState, len(machines))
	for _, ms := range machines {
		ms := ms
		agents[ms.ID] = agent.NewAgentState(&ms)
	}

	for _, j := range jobs {
		j := j

		if !j.Scheduled() {
			continue
		}

		as := agents[j.TargetMachineID]
		if as == nil {
			continue
		}

		as.Jobs[j.Name] = &j
	}

	return &clusterState{
		jobs:   jobs,
		agents: agents,
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

func (cs *clusterState) machineExists(machID string) bool {
	_, ok := cs.agents[machID]
	return ok
}
