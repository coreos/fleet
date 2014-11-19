/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package engine

import (
	"fmt"
	"sort"

	"github.com/coreos/fleet/agent"
	"github.com/coreos/fleet/job"

	"github.com/coreos/fleet/log"
)

type decision struct {
	machineID string
}

type Scheduler interface {
	Decide(*clusterState, *job.Job) (*decision, error)
}

var (
	defaultSchedulerMap = map[string]Scheduler{
		"LeastLoaded":          &sortingAgentScheduler{&leastLoadedSorter{}},
		"LeastSystemStatField": &sortingAgentScheduler{&leastSystemStatFieldSorter{}},
	}
)

// choose a Scheduler via unit file X-Fleet Scheduler
type selectingScheduler struct {
	schedulerMap map[string]Scheduler
}

func (ss *selectingScheduler) Decide(clust *clusterState, j *job.Job) (*decision, error) {

	// default Scheduler
	choosedSchedulerName := "LeastLoaded"

	var choosedScheduler Scheduler = defaultSchedulerMap[choosedSchedulerName]

	if schedulerName, ok := j.RequiredScheduler(); ok {
		if scheduler, ok := ss.schedulerMap[schedulerName]; ok {
			choosedScheduler = scheduler
			choosedSchedulerName = schedulerName
		}
	}

	log.Infof("Using %s for job %s", choosedSchedulerName, j.Name)
	return choosedScheduler.Decide(clust, j)
}

type agentSorter interface {
	sortedAgents(sas sortableAgentStates, j *job.Job) []*agent.AgentState
}

type sortingAgentScheduler struct {
	agentSorter
}

func makeSas(clust *clusterState) sortableAgentStates {
	agents := clust.agents()

	sas := make(sortableAgentStates, 0)
	for _, as := range agents {
		sas = append(sas, as)
	}

	return sas
}

func (s *sortingAgentScheduler) Decide(clust *clusterState, j *job.Job) (*decision, error) {
	sas := makeSas(clust)
	agents := s.agentSorter.sortedAgents(sas, j)

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

type leastLoadedSorter struct{}

// sortedAgents returns a list of AgentState objects sorted ascending
// by the number of scheduled units
func (lls *leastLoadedSorter) sortedAgents(sas sortableAgentStates, j *job.Job) []*agent.AgentState {
	sort.Sort(byLoadedUnitCount{sas})

	return []*agent.AgentState(sas)
}

type leastSystemStatFieldSorter struct{}

func (lssfs *leastSystemStatFieldSorter) sortedAgents(sas sortableAgentStates, j *job.Job) []*agent.AgentState {
	sort.Sort(lssfs.chooseDimension(sas, j))

	return []*agent.AgentState(sas)
}

func (lssfs *leastSystemStatFieldSorter) chooseDimension(sas sortableAgentStates, j *job.Job) sort.Interface {

	sortfield := "load5" // default filed

	metadata := j.RequiredSchedulerMetadata()
	if fields, ok := metadata["sysstatfield"]; ok && fields.Length() > 0 {
		sortfield = fields.Values()[0]
	}

	log.Infof("LeastSystemStatFieldScheduler sort machines by sysstat field %s for job %s", sortfield, j.Name)

	return bySysstatField{sas, sortfield}
}

type sortableAgentStates []*agent.AgentState

func (sas sortableAgentStates) Len() int      { return len(sas) }
func (sas sortableAgentStates) Swap(i, j int) { sas[i], sas[j] = sas[j], sas[i] }

type byLoadedUnitCount struct{ sortableAgentStates }

func (sas byLoadedUnitCount) Less(i, j int) bool {

	niUnits := len(sas.sortableAgentStates[i].Units)
	njUnits := len(sas.sortableAgentStates[j].Units)
	return niUnits < njUnits || (niUnits == njUnits && sas.sortableAgentStates[i].MState.ID < sas.sortableAgentStates[j].MState.ID)
}

func statFieldLess(sas sortableAgentStates, field string, i, j int) bool {

	ni, oki := sas[i].MState.Statdata[field]
	nj, okj := sas[j].MState.Statdata[field]

	if oki && okj {
		return ni < nj || (ni == nj && sas[i].MState.ID < sas[j].MState.ID)
	} else if oki {
		return true
	} else if okj {
		return false
	}

	return sas[i].MState.ID < sas[j].MState.ID
}

type bySysstatField struct {
	sortableAgentStates
	field string
}

func (sas bySysstatField) Less(i, j int) bool {
	return statFieldLess(sas.sortableAgentStates, sas.field, i, j)
}
