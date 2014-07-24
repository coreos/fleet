package agent

import (
	"encoding/json"
	"sync"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/job"
)

type AgentCache struct {
	// used to lock the datastructure for multi-goroutine safety
	mutex sync.Mutex

	// expected states of jobs scheduled to this agent
	targetStates map[string]job.JobState
}

func NewCache() *AgentCache {
	return &AgentCache{
		targetStates: make(map[string]job.JobState),
	}
}

func (ac *AgentCache) Lock() {
	log.V(1).Infof("Attempting to lock AgentCache")
	ac.mutex.Lock()
	log.V(1).Infof("AgentCache locked")
}

func (ac *AgentCache) Unlock() {
	log.V(1).Infof("Attempting to unlock AgentCache")
	ac.mutex.Unlock()
	log.V(1).Infof("AgentCache unlocked")
}

func (ac *AgentCache) MarshalJSON() ([]byte, error) {
	type ds struct {
		TargetStates map[string]job.JobState
	}
	data := ds{
		TargetStates: ac.targetStates,
	}
	return json.Marshal(data)
}


// PurgeJob removes all state tracked on behalf of a given job
func (ac *AgentCache) PurgeJob(jobName string) {
	ac.dropTargetState(jobName)
}

func (ac *AgentCache) SetTargetState(jobName string, state job.JobState) {
	ac.targetStates[jobName] = state
}

func (ac *AgentCache) dropTargetState(jobName string) {
	delete(ac.targetStates, jobName)
}

func (ac *AgentCache) LaunchedJobs() []string {
	jobs := make([]string, 0)
	for j, ts := range ac.targetStates {
		if ts == job.JobStateLaunched {
			jobs = append(jobs, j)
		}
	}
	return jobs
}

func (ac *AgentCache) ScheduledJobs() []string {
	jobs := make([]string, 0)
	for j, ts := range ac.targetStates {
		if ts == job.JobStateLoaded || ts == job.JobStateLaunched {
			jobs = append(jobs, j)
		}
	}
	return jobs
}

func (ac *AgentCache) ScheduledHere(jobName string) bool {
	ts := ac.targetStates[jobName]
	return ts == job.JobStateLoaded || ts == job.JobStateLaunched
}
