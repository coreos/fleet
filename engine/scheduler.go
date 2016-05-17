// Copyright 2014 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

func computeAgentScore(a *agent.AgentState) float64 {
	weightSum := 0

	for _, u := range a.Units {
		weightSum += u.Unit.Weight()
	}

	return float64(weightSum) / float64(a.MState.Shares())
}

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
// by the computed score
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
	niScore := computeAgentScore(sas[i])
	njScore := computeAgentScore(sas[j])

	if niScore != njScore {
		return niScore < njScore
	}

	if sas[i].MState.Shares() != sas[j].MState.Shares() {
		return sas[i].MState.Shares() > sas[j].MState.Shares()
	}

	return sas[i].MState.ID < sas[j].MState.ID
}
