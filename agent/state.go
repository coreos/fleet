package agent

import (
	"path"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/job"
)

type agentState struct {
	jobs map[string]*job.Job
}

func newAgentState() *agentState {
	return &agentState{jobs: make(map[string]*job.Job)}
}

func (as *agentState) jobScheduled(jName string) bool {
	return as.jobs[jName] != nil
}

// hasConflict determines whether there are any known conflicts with the given Job
func (as *agentState) hasConflict(pJobName string, pConflicts []string) (found bool, conflict string) {
	for _, eJob := range as.jobs {
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
