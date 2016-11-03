// Copyright 2014 The fleet Authors
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

package agent

import (
	"fmt"
	"path"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/log"
	"github.com/coreos/fleet/machine"
)

type AgentState struct {
	MState *machine.MachineState
	Units  map[string]*job.Unit
}

func NewAgentState(ms *machine.MachineState) *AgentState {
	return &AgentState{
		MState: ms,
		Units:  make(map[string]*job.Unit),
	}
}

func (as *AgentState) unitScheduled(name string) bool {
	return as.Units[name] != nil
}

// HasConflict determines whether there are any known conflicts with the given Unit
func (as *AgentState) HasConflict(pUnitName string, pConflicts []string) (found bool, conflict string) {
	for _, eUnit := range as.Units {
		if pUnitName == eUnit.Name {
			continue
		}

		for _, pConflict := range pConflicts {
			if globMatches(pConflict, eUnit.Name) {
				found = true
				conflict = eUnit.Name
				return
			}
		}

		for _, eConflict := range eUnit.Conflicts() {
			if globMatches(eConflict, pUnitName) {
				found = true
				conflict = eUnit.Name
				return
			}
		}
	}

	return
}

// hasReplace determines whether there are any known replaces with the given Unit
func (as *AgentState) hasReplace(pUnitName string, pReplaces []string) (found bool, replace string) {
	for _, eUnit := range as.Units {
		foundPrepl := false
		foundErepl := false
		retStr := ""

		if pUnitName == eUnit.Name {
			continue
		}

		for _, pReplace := range pReplaces {
			if globMatches(pReplace, eUnit.Name) {
				foundPrepl = true
				retStr = eUnit.Name
				break
			}
		}

		for _, eReplace := range eUnit.Replaces() {
			if globMatches(eReplace, pUnitName) {
				foundErepl = true
				retStr = eUnit.Name
				break
			}
		}

		// Only 1 of 2 matches must be found. If both matches are found,
		// it means it's a circular replace situation, which could result in
		// an infinite loop. So ignore such replace options.
		if (foundPrepl && foundErepl) || (!foundPrepl && !foundErepl) {
			continue
		} else {
			found = true
			replace = retStr
			return
		}
	}

	return
}

func globMatches(pattern, target string) bool {
	matched, err := path.Match(pattern, target)
	if err != nil {
		log.Debugf("Received error while matching pattern '%s': %v", pattern, err)
	}
	return matched
}

// AbleToRun determines if an Agent can run the provided Job based on
// the Agent's current state. A boolean indicating whether this is the
// case or not is returned. The following criteria is used:
//   - Agent must meet the Job's machine target requirement (if any)
//   - Agent must have all of the Job's required metadata (if any)
//   - Agent must have all required Peers of the Job scheduled locally (if any)
//   - Job must not conflict with any other Units scheduled to the agent
//   - Job must specially handle replaced units to be rescheduled
func (as *AgentState) AbleToRun(j *job.Job) (jobAction job.JobAction, errstr string) {
	if tgt, ok := j.RequiredTarget(); ok && !as.MState.MatchID(tgt) {
		return job.JobActionUnschedule, fmt.Sprintf("agent ID %q does not match required %q", as.MState.ID, tgt)
	}

	metadata := j.RequiredTargetMetadata()
	if len(metadata) != 0 {
		if !machine.HasMetadata(as.MState, metadata) {
			return job.JobActionUnschedule, "local Machine metadata insufficient"
		}
	}

	peers := j.Peers()
	if len(peers) != 0 {
		for _, peer := range peers {
			if !as.unitScheduled(peer) {
				return job.JobActionUnschedule, fmt.Sprintf("required peer Unit(%s) is not scheduled locally", peer)
			}
		}
	}

	if cExists, cJobName := as.HasConflict(j.Name, j.Conflicts()); cExists {
		return job.JobActionUnschedule, fmt.Sprintf("found conflict with locally-scheduled Unit(%s)", cJobName)
	}

	// Handle Replace option specially for rescheduling the unit
	if cExists, cJobName := as.hasReplace(j.Name, j.Replaces()); cExists {
		return job.JobActionReschedule, fmt.Sprintf("found replace with locally-scheduled Unit(%s)", cJobName)
	}

	return job.JobActionSchedule, ""
}
