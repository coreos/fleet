package agent

import (
	"encoding/json"
	"sync"

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
