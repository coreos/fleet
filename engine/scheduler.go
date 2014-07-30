package engine

import (
	"fmt"
	"sort"

	"github.com/coreos/fleet/agent"
	"github.com/coreos/fleet/job"
)

type decision struct {
	machineID string
}

type Scheduler interface {
	Decide(*clusterState, *job.Job) (*decision, error)
}

type leastLoadedScheduler struct{}

func (lls *leastLoadedScheduler) Decide(clust *clusterState, j *job.Job) (*decision, error) {
	agents := lls.sortedAgents(clust)

	if len(agents) == 0 {
		return nil, fmt.Errorf("zero agents available")
	}

	var target *agent.AgentState
	for _, as := range agents {
		if able, _ := as.AbleToRun(j); !able {
			continue
		}

		as := as
		target = as
		break
	}

	if target == nil {
		return nil, fmt.Errorf("no agents able to run job")
	}

	dec := decision{
		machineID: target.MState.ID,
	}

	return &dec, nil
}

// sortedAgents returns a list of AgentState objects sorted ascending
// by the number of scheduled units
func (lls *leastLoadedScheduler) sortedAgents(clust *clusterState) []*agent.AgentState {
	agents := clust.agents()

	sas := make(sortableAgentStates, 0)
	for _, as := range agents {
		sas = append(sas, as)
	}
	sort.Sort(sas)

	return []*agent.AgentState(sas)
}

type sortableAgentStates []*agent.AgentState

func (sas sortableAgentStates) Len() int      { return len(sas) }
func (sas sortableAgentStates) Swap(i, j int) { sas[i], sas[j] = sas[j], sas[i] }

func (sas sortableAgentStates) Less(i, j int) bool {
	niJobs := len(sas[i].Jobs)
	njJobs := len(sas[j].Jobs)
	return niJobs < njJobs || (niJobs == njJobs && sas[i].MState.ID < sas[j].MState.ID)
}
