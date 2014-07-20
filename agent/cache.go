package agent

import (
	"encoding/json"
	"sync"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/resource"
)

type AgentCache struct {
	// used to lock the datastructure for multi-goroutine safety
	mutex sync.Mutex

	// expected states of jobs scheduled to this agent
	targetStates map[string]job.JobState

	// resources by job
	// TODO(uwedeportivo): this is temporary until we derive this from systemd
	// systemd will give us useful info even for jobs that didn't declare resource reservations
	resources map[string]resource.ResourceTuple
}

func NewCache() *AgentCache {
	return &AgentCache{
		targetStates: make(map[string]job.JobState),
		resources:    make(map[string]resource.ResourceTuple),
	}
}

func (as *AgentCache) Lock() {
	log.V(1).Infof("Attempting to lock AgentCache")
	as.mutex.Lock()
	log.V(1).Infof("AgentCache locked")
}

func (as *AgentCache) Unlock() {
	log.V(1).Infof("Attempting to unlock AgentCache")
	as.mutex.Unlock()
	log.V(1).Infof("AgentCache unlocked")
}

func (as *AgentCache) MarshalJSON() ([]byte, error) {
	type ds struct {
		TargetStates map[string]job.JobState
	}
	data := ds{
		TargetStates: as.targetStates,
	}
	return json.Marshal(data)
}

// TrackJob extracts and stores information about the given job for later reference
func (as *AgentCache) TrackJob(j *job.Job) {
	as.trackJobResources(j.Name, j.Resources())
}

// PurgeJob removes all state tracked on behalf of a given job
func (as *AgentCache) PurgeJob(jobName string) {
	as.dropTargetState(jobName)
	as.dropJobResources(jobName)
}

func (as *AgentCache) trackJobResources(jobName string, res resource.ResourceTuple) {
	as.resources[jobName] = res
}

func (as *AgentCache) dropJobResources(jobName string) {
	delete(as.resources, jobName)
}

func (as *AgentCache) SetTargetState(jobName string, state job.JobState) {
	as.targetStates[jobName] = state
}

func (as *AgentCache) dropTargetState(jobName string) {
	delete(as.targetStates, jobName)
}

func (as *AgentCache) LaunchedJobs() []string {
	jobs := make([]string, 0)
	for j, ts := range as.targetStates {
		if ts == job.JobStateLaunched {
			jobs = append(jobs, j)
		}
	}
	return jobs
}

func (as *AgentCache) ScheduledJobs() []string {
	jobs := make([]string, 0)
	for j, ts := range as.targetStates {
		if ts == job.JobStateLoaded || ts == job.JobStateLaunched {
			jobs = append(jobs, j)
		}
	}
	return jobs
}

func (as *AgentCache) ScheduledHere(jobName string) bool {
	ts := as.targetStates[jobName]
	return ts == job.JobStateLoaded || ts == job.JobStateLaunched
}
