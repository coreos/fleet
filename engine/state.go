package engine

import (
	"github.com/coreos/fleet/agent"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
)

type clusterState struct {
	jobs     map[string]*job.Job
	gUnits   map[string]*job.Unit
	machines map[string]*machine.MachineState
}

func newClusterState(units []job.Unit, sUnits []job.ScheduledUnit, machines []machine.MachineState) *clusterState {
	sUnitMap := make(map[string]*job.ScheduledUnit)
	for _, sUnit := range sUnits {
		sUnit := sUnit
		sUnitMap[sUnit.Name] = &sUnit
	}

	jMap := make(map[string]*job.Job)
	guMap := make(map[string]*job.Unit)
	for _, u := range units {
		if u.IsGlobal() {
			u := u
			guMap[u.Name] = &u
		} else {
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
	}

	mMap := make(map[string]*machine.MachineState, len(machines))
	for _, ms := range machines {
		ms := ms
		mMap[ms.ID] = &ms
	}

	return &clusterState{
		jobs:     jMap,
		gUnits:   guMap,
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
		j := j
		if !j.Scheduled() || j.TargetState == job.JobStateInactive {
			continue
		}
		if as, ok := agents[j.TargetMachineID]; ok {
			as.Jobs[j.Name] = j
		}
	}

	for _, gu := range cs.gUnits {
		j := &job.Job{
			Name:        gu.Name,
			Unit:        gu.Unit,
			TargetState: gu.TargetState,
		}
		for _, a := range agents {
			a.Jobs[gu.Name] = j
		}
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
