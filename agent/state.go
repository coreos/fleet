package agent

import (
	"fmt"
	"path"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
)

type AgentState struct {
	MState *machine.MachineState
	Jobs   map[string]*job.Job

	// This is used to assert that the Agent is able to
	// run a given job based on its signature. This feature
	// is currently deprecated, so expect this to go away.
	verifyFunc func(j *job.Job) bool
}

func NewAgentState(ms *machine.MachineState) *AgentState {
	return &AgentState{
		MState: ms,
		Jobs:   make(map[string]*job.Job),
	}
}

func (as *AgentState) jobScheduled(jName string) bool {
	return as.Jobs[jName] != nil
}

// hasConflict determines whether there are any known conflicts with the given Job
func (as *AgentState) hasConflict(pJobName string, pConflicts []string) (found bool, conflict string) {
	for _, eJob := range as.Jobs {
		if pJobName == eJob.Name {
			continue
		}

		for _, pConflict := range pConflicts {
			if globMatches(pConflict, eJob.Name) {
				found = true
				conflict = eJob.Name
				return
			}
		}

		for _, eConflict := range eJob.Conflicts() {
			if globMatches(eConflict, pJobName) {
				found = true
				conflict = eJob.Name
				return
			}
		}
	}

	return
}

func globMatches(pattern, target string) bool {
	matched, err := path.Match(pattern, target)
	if err != nil {
		log.V(1).Infof("Received error while matching pattern '%s': %v", pattern, err)
	}
	return matched
}

// AbleToRun determines if an Agent can run the provided Job based on
// the Agent's current state. A boolean indicating whether this is the
// case or not is returned. The following criteria is used:
//   - Agent must meet the Job's machine target requirement (if any)
//   - Job must pass signature verification
//   - Agent must have all of the Job's required metadata (if any)
//   - Agent must have all required Peers of the Job scheduled locally (if any)
//   - Job must not conflict with any other Jobs scheduled to the agent
func (as *AgentState) AbleToRun(j *job.Job) (bool, string) {
	if tgt, ok := j.RequiredTarget(); ok && !as.MState.MatchID(tgt) {
		return false, fmt.Sprintf("agent ID %q does not match required %q", as.MState.ID, tgt)
	}

	if as.verifyFunc != nil && !as.verifyFunc(j) {
		return false, "unable to verify signature"
	}

	metadata := j.RequiredTargetMetadata()
	if len(metadata) != 0 {
		if !machine.HasMetadata(as.MState, metadata) {
			return false, "local Machine metadata insufficient"
		}
	}

	peers := j.Peers()
	if len(peers) != 0 {
		for _, peer := range peers {
			if !as.jobScheduled(peer) {
				return false, fmt.Sprintf("required peer Job(%s) is not scheduled locally", peer)
			}
		}
	}

	if cExists, cJobName := as.hasConflict(j.Name, j.Conflicts()); cExists {
		return false, fmt.Sprintf("found conflict with locally-scheduled Job(%s)", cJobName)
	}

	return true, ""
}
