package agent

import (
	"encoding/json"
	"path"
	"sync"

	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/resource"
)

type AgentState struct {
	// used to lock the datastructure for multi-goroutine safety
	mutex sync.Mutex

	// unresolved job offers
	offers map[string]job.JobOffer

	// job names for which a bid has been submitted
	bids map[string]bool

	// reverse index of peers that would cause a reassesment of a JobOffer this
	// Agent could not have bid on previously
	// i.e. {"hello.service": ["howareyou.service", "goodbye.service"]}
	peers map[string][]string

	// index of local payload conflicts to the job they belong to
	Conflicts map[string][]string

	// expected states of jobs scheduled to this agent
	targetStates map[string]job.JobState

	// resources by job
	// TODO(uwedeportivo): this is temporary until we derive this from systemd
	// systemd will give us useful info even for jobs that didn't declare resource reservations
	resources map[string]resource.ResourceTuple
}

func NewState() *AgentState {
	return &AgentState{
		offers:       make(map[string]job.JobOffer),
		bids:         make(map[string]bool),
		peers:        make(map[string][]string),
		Conflicts:    make(map[string][]string, 0),
		targetStates: make(map[string]job.JobState),
		resources:    make(map[string]resource.ResourceTuple),
	}
}

func (as *AgentState) Lock() {
	log.V(1).Infof("Attempting to lock AgentState")
	as.mutex.Lock()
	log.V(1).Infof("AgentState locked")
}

func (as *AgentState) Unlock() {
	log.V(1).Infof("Attempting to unlock AgentState")
	as.mutex.Unlock()
	log.V(1).Infof("AgentState unlocked")
}

func (as *AgentState) MarshalJSON() ([]byte, error) {
	type ds struct {
		Offers       map[string]job.JobOffer
		Conflicts    map[string][]string
		Bids         map[string]bool
		Peers        map[string][]string
		TargetStates map[string]job.JobState
	}
	data := ds{
		Offers:       as.offers,
		Conflicts:    as.Conflicts,
		Bids:         as.bids,
		Peers:        as.peers,
		TargetStates: as.targetStates,
	}
	return json.Marshal(data)
}

// TrackJob extracts and stores information about the given job for later reference
func (as *AgentState) TrackJob(j *job.Job) {
	as.trackJobPeers(j.Name, j.Peers())
	as.trackJobConflicts(j.Name, j.Conflicts())
	as.trackJobResources(j.Name, j.Resources())
}

// PurgeJob removes all state tracked on behalf of a given job
func (as *AgentState) PurgeJob(jobName string) {
	as.dropTargetState(jobName)
	as.dropPeersJob(jobName)
	as.dropJobConflicts(jobName)
	as.dropJobResources(jobName)
}

func (as *AgentState) trackJobConflicts(jobName string, conflicts []string) {
	as.Conflicts[jobName] = conflicts
}

// Purge all tracked conflicts for a given Job
func (as *AgentState) dropJobConflicts(jobName string) {
	delete(as.Conflicts, jobName)
}

// Store a relation of 1 Job -> N Peers
func (as *AgentState) trackJobPeers(jobName string, peers []string) {
	for _, peer := range peers {
		_, ok := as.peers[peer]
		if !ok {
			as.peers[peer] = make([]string, 0)
		}
		as.peers[peer] = append(as.peers[peer], jobName)
	}
}

func (as *AgentState) trackJobResources(jobName string, res resource.ResourceTuple) {
	as.resources[jobName] = res
}

func (as *AgentState) dropJobResources(jobName string) {
	delete(as.resources, jobName)
}

// Retrieve all Jobs that share a given Peer
func (as *AgentState) GetJobsByPeer(peerName string) []string {
	peers, ok := as.peers[peerName]
	if ok {
		return peers
	}
	return make([]string, 0)
}

// Remove all references to a given Job from all Peer indexes
func (as *AgentState) dropPeersJob(jobName string) {
	for peer, peerIndex := range as.peers {
		var idxs []int

		// Determine which item indexes must be removed from the Peer index
		for idx, record := range peerIndex {
			if jobName == record {
				idxs = append(idxs, idx)
			}
		}

		// Iterate through the item indexes, removing the corresponding Peers
		for i, idx := range idxs {
			as.peers[peer] = append(as.peers[peer][0:idx-i], as.peers[peer][idx-i+1:]...)
		}

		// Clean up empty peer relations when possible
		if len(as.peers[peer]) == 0 {
			delete(as.peers, peer)
		}
	}
}

func (as *AgentState) TrackOffer(offer job.JobOffer) {
	as.offers[offer.Job.Name] = offer
}

// GetOffersWithoutBids returns all tracked JobOffers that have
// no corresponding JobBid tracked in the same AgentState object.
func (as *AgentState) GetOffersWithoutBids() []job.JobOffer {
	offers := make([]job.JobOffer, 0)
	for _, offer := range as.offers {
		if !as.bids[offer.Job.Name] {
			offers = append(offers, offer)
		}
	}
	return offers
}

func (as *AgentState) PurgeOffer(name string) {
	delete(as.offers, name)
	delete(as.bids, name)
}

func (as *AgentState) TrackBid(name string) {
	as.bids[name] = true
}

func (as *AgentState) HasBid(name string) bool {
	return as.bids[name]
}

func globMatches(pattern, target string) bool {
	matched, err := path.Match(pattern, target)
	if err != nil {
		log.V(1).Infof("Received error while matching pattern '%s': %v", pattern, err)
	}
	return matched
}

func (as *AgentState) SetTargetState(jobName string, state job.JobState) {
	as.targetStates[jobName] = state
}

func (as *AgentState) dropTargetState(jobName string) {
	delete(as.targetStates, jobName)
}

// StartedJobs returns a list of Job names that have been started by the agent
func (as *AgentState) StartedJobs() []string {
	jobs := make([]string, 0)
	for j, ts := range as.targetStates {
		if isStarted(ts) {
			jobs = append(jobs, j)
		}
	}
	return jobs
}

// ScheduledJobs returns a list of Job names that are currently scheduled to the agent
func (as *AgentState) ScheduledJobs() []string {
	jobs := make([]string, 0)
	for j, ts := range as.targetStates {
		if isScheduled(ts) {
			jobs = append(jobs, j)
		}
	}
	return jobs
}

// ScheduledHere returns whether the Job by the given name is scheduled to the agent
func (as *AgentState) ScheduledHere(jobName string) bool {
	return isScheduled(as.targetStates[jobName])
}

func isScheduled(ts job.JobState) bool {
	return ts == job.JobStateLoaded || ts == job.JobStateLaunched || ts == job.JobStateCompleted
}

func isStarted(ts job.JobState) bool {
	return ts == job.JobStateLaunched || ts == job.JobStateCompleted
}
