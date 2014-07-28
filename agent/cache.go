package agent

import (
	"encoding/json"

	"github.com/coreos/fleet/job"
)

type agentCache map[string]job.JobState

func (ac *agentCache) MarshalJSON() ([]byte, error) {
	type ds struct {
		TargetStates map[string]job.JobState
	}
	data := ds{
		TargetStates: map[string]job.JobState(*ac),
	}
	return json.Marshal(data)
}

func (ac *agentCache) setTargetState(jobName string, state job.JobState) {
	(*ac)[jobName] = state
}

func (ac *agentCache) dropTargetState(jobName string) {
	delete(*ac, jobName)
}

func (ac *agentCache) launchedJobs() []string {
	jobs := make([]string, 0)
	for j, ts := range *ac {
		if ts == job.JobStateLaunched {
			jobs = append(jobs, j)
		}
	}
	return jobs
}

func (ac *agentCache) loadedJobs() []string {
	jobs := make([]string, 0)
	for j, ts := range *ac {
		if ts == job.JobStateLoaded {
			jobs = append(jobs, j)
		}
	}
	return jobs
}
