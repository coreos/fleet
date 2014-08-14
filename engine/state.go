package engine

import (
	"github.com/coreos/fleet/agent"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
)

type clusterState struct {
	jobs     map[string]*job.Job
	machines map[string]*machine.MachineState
}

func newClusterState(units []job.Unit, sUnits []job.ScheduledUnit, machines []machine.MachineState) *clusterState {
	sUnitMap := make(map[string]*job.ScheduledUnit)
	for _, sUnit := range sUnits {
		sUnit := sUnit
		sUnitMap[sUnit.Name] = &sUnit
	}

	jMap := make(map[string]*job.Job, len(units))
	for _, u := range units {
		j := job.Job{
			Name:        u.Name,
			Unit:        u.Unit,
			TargetState: u.TargetState,
		}

		if sUnit, ok := sUnitMap[u.Name]; ok {
			j.TargetMachineID = sUnit.TargetMachineID
			j.State = sUnit.State
		}

		jMap[j.Name] = &j
	}

	mMap := make(map[string]*machine.MachineState, len(machines))
	for _, ms := range machines {
		ms := ms
		mMap[ms.ID] = &ms
	}

	return &clusterState{
		jobs:     jMap,
		machines: mMap,
	}
}

func (cs *clusterState) agents() map[string]*agent.AgentState {
	agents := make(map[string]*agent.AgentState, len(cs.machines))
	for _, ms := range cs.machines {
		ms := ms
		agents[ms.ID] = agent.NewAgentState(ms)
	}

	for _, j := range cs.jobs {
		if !j.Scheduled() || j.TargetState == job.JobStateInactive {
			continue
		}

		as := agents[j.TargetMachineID]
		if as == nil {
			continue
		}

		j := j
		as.Jobs[j.Name] = j
	}

	return agents
}

func (cs *clusterState) schedule(jobName, targetMachineID string) {
	j := cs.jobs[jobName]
	if j == nil {
		return
	}
	j.TargetMachineID = targetMachineID
}

func (cs *clusterState) unschedule(jobName string) {
	j := cs.jobs[jobName]
	if j == nil {
		return
	}
	j.TargetMachineID = ""
}
