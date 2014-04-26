package agent

import (
	"encoding/json"
	"path"
	"sync"

	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/job"
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
}

func NewState() *AgentState {
	return &AgentState{
		offers:       make(map[string]job.JobOffer),
		bids:         make(map[string]bool),
		peers:        make(map[string][]string),
		Conflicts:    make(map[string][]string, 0),
		targetStates: make(map[string]job.JobState),
	}
}

func (self *AgentState) lock() {
	log.V(1).Infof("Attempting to lock AgentState")
	self.mutex.Lock()
	log.V(1).Infof("AgentState locked")
}

func (self *AgentState) unlock() {
	log.V(1).Infof("Attempting to unlock AgentState")
	self.mutex.Unlock()
	log.V(1).Infof("AgentState unlocked")
}

func (self *AgentState) MarshalJSON() ([]byte, error) {
	type ds struct {
		Offers       map[string]job.JobOffer
		Conflicts    map[string][]string
		Bids         map[string]bool
		Peers        map[string][]string
		TargetStates map[string]job.JobState
	}
	data := ds{
		Offers:       self.offers,
		Conflicts:    self.Conflicts,
		Bids:         self.bids,
		Peers:        self.peers,
		TargetStates: self.targetStates,
	}
	return json.Marshal(data)
}

// TrackJob extracts and stores information about the given job for later reference
func (self *AgentState) TrackJob(j *job.Job) {
	self.lock()
	defer self.unlock()

	self.trackJobPeers(j.Name, j.Payload.Peers())
	self.trackJobConflicts(j.Name, j.Payload.Conflicts())
}

// PurgeJob removes all state tracked on behalf of a given job
func (self *AgentState) PurgeJob(jobName string) {
	self.lock()
	defer self.unlock()

	self.dropTargetState(jobName)
	self.dropPeersJob(jobName)
	self.dropJobConflicts(jobName)
}

func (self *AgentState) trackJobConflicts(jobName string, conflicts []string) {
	self.Conflicts[jobName] = conflicts
}

// Purge all tracked conflicts for a given Job
func (self *AgentState) dropJobConflicts(jobName string) {
	delete(self.Conflicts, jobName)
}

// Store a relation of 1 Job -> N Peers
func (self *AgentState) trackJobPeers(jobName string, peers []string) {
	for _, peer := range peers {
		_, ok := self.peers[peer]
		if !ok {
			self.peers[peer] = make([]string, 0)
		}
		self.peers[peer] = append(self.peers[peer], jobName)
	}
}

// Retrieve all Jobs that share a given Peer
func (self *AgentState) GetJobsByPeer(peerName string) []string {
	self.lock()
	defer self.unlock()

	peers, ok := self.peers[peerName]
	if ok {
		return peers
	} else {
		return make([]string, 0)
	}
}

// Remove all references to a given Job from all Peer indexes
func (self *AgentState) dropPeersJob(jobName string) {
	for peer, peerIndex := range self.peers {
		var idxs []int

		// Determine which item indexes must be removed from the Peer index
		for idx, record := range peerIndex {
			if jobName == record {
				idxs = append(idxs, idx)
			}
		}

		// Iterate through the item indexes, removing the corresponding Peers
		for i, idx := range idxs {
			self.peers[peer] = append(self.peers[peer][0:idx-i], self.peers[peer][idx-i+1:]...)
		}

		// Clean up empty peer relations when possible
		if len(self.peers[peer]) == 0 {
			delete(self.peers, peer)
		}
	}
}

func (self *AgentState) TrackOffer(offer job.JobOffer) {
	self.lock()
	defer self.unlock()

	self.offers[offer.Job.Name] = offer
}

// GetOffersWithoutBids returns all tracked JobOffers that have
// no corresponding JobBid tracked in the same AgentState object.
func (self *AgentState) GetOffersWithoutBids() []job.JobOffer {
	self.lock()
	defer self.unlock()

	offers := make([]job.JobOffer, 0)
	for _, offer := range self.offers {
		if !self.bids[offer.Job.Name] {
			offers = append(offers, offer)
		}
	}
	return offers
}

func (self *AgentState) PurgeOffer(name string) {
	self.lock()
	defer self.unlock()

	delete(self.offers, name)
	delete(self.bids, name)
}

func (self *AgentState) TrackBid(name string) {
	self.lock()
	defer self.unlock()

	self.bids[name] = true
}

func (self *AgentState) HasBid(name string) bool {
	self.lock()
	defer self.unlock()

	return self.bids[name]
}

func globMatches(pattern, target string) bool {
	matched, err := path.Match(pattern, target)
	if err != nil {
		log.V(1).Infof("Received error while matching pattern '%s': %v", pattern, err)
	}
	return matched
}

func (self *AgentState) SetTargetState(jobName string, state job.JobState) {
	self.lock()
	defer self.unlock()

	self.targetStates[jobName] = state
}

func (self *AgentState) dropTargetState(jobName string) {
	delete(self.targetStates, jobName)
}

func (self *AgentState) LaunchedJobs() []string {
	self.lock()
	defer self.unlock()

	jobs := make([]string, 0)
	for j, ts := range self.targetStates {
		if ts == job.JobStateLaunched {
			jobs = append(jobs, j)
		}
	}
	return jobs
}

func (self *AgentState) ScheduledJobs() []string {
	self.lock()
	defer self.unlock()

	jobs := make([]string, 0)
	for j, ts := range self.targetStates {
		if ts == job.JobStateLoaded || ts == job.JobStateLaunched {
			jobs = append(jobs, j)
		}
	}
	return jobs
}

func (self *AgentState) ScheduledHere(jobName string) bool {
	self.lock()
	defer self.unlock()

	ts := self.targetStates[jobName]
	return ts == job.JobStateLoaded || ts == job.JobStateLaunched
}
