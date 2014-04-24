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
	conflicts map[string][]string

	// expected states of jobs scheduled to this agent
	targetStates map[string]job.JobState
}

func NewState() *AgentState {
	return &AgentState{
		offers:       make(map[string]job.JobOffer),
		bids:         make(map[string]bool),
		peers:        make(map[string][]string),
		conflicts:    make(map[string][]string, 0),
		targetStates: make(map[string]job.JobState),
	}
}

func (self *AgentState) Lock() {
	log.V(2).Infof("Attempting to lock AgentState")
	self.mutex.Lock()
	log.V(2).Infof("AgentState locked")
}

func (self *AgentState) Unlock() {
	log.V(2).Infof("Attempting to unlock AgentState")
	self.mutex.Unlock()
	log.V(2).Infof("AgentState unlocked")
}

func (self *AgentState) MarshalJSON() ([]byte, error) {
	type ds struct {
		Offers    map[string]job.JobOffer
		Conflicts map[string][]string
		Bids      map[string]bool
		Peers     map[string][]string
	}
	data := ds{
		Offers:    self.offers,
		Conflicts: self.conflicts,
		Bids:      self.bids,
		Peers:     self.peers,
	}
	return json.Marshal(data)
}

// Store a list of conflicts on behalf of a given Job
func (self *AgentState) TrackJobConflicts(jobName string, conflicts []string) {
	self.conflicts[jobName] = conflicts
}

// Determine whether there are any known conflicts with the given argument
func (self *AgentState) HasConflict(potentialJobName string, potentialConflicts []string) (bool, string) {
	// Iterate through each Job that is scheduled here or has already been bid upon, asserting two things
	for existingJobName, existingConflicts := range self.conflicts {
		if !self.HasBid(existingJobName) && !self.ScheduledHere(existingJobName) {
			continue
		}

		// 1. Each tracked Job does not conflict with the potential conflicts
		for _, pc := range potentialConflicts {
			if globMatches(pc, existingJobName) {
				return true, existingJobName
			}
		}

		// 2. The new Job does not conflict with any of the tracked confclits
		for _, ec := range existingConflicts {
			if globMatches(ec, potentialJobName) {
				return true, existingJobName
			}
		}
	}

	return false, ""
}

// Purge all tracked conflicts for a given Job
func (self *AgentState) DropJobConflicts(jobName string) {
	delete(self.conflicts, jobName)
}

// Store a relation of 1 Job -> N Peers
func (self *AgentState) TrackJobPeers(jobName string, peers []string) {
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
	peers, ok := self.peers[peerName]
	if ok {
		return peers
	} else {
		return make([]string, 0)
	}
}

// Remove all references to a given Job from all Peer indexes
func (self *AgentState) DropPeersJob(jobName string) {
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
	self.offers[offer.Job.Name] = offer
}

// GetOffersWithoutBids returns all tracked JobOffers that have
// no corresponding JobBid tracked in the same AgentState object.
func (self *AgentState) GetOffersWithoutBids() []job.JobOffer {
	offers := make([]job.JobOffer, 0)
	for _, offer := range self.offers {
		if !self.HasBid(offer.Job.Name) {
			offers = append(offers, offer)
		}
	}
	return offers
}

func (self *AgentState) DropOffer(name string) {
	if _, ok := self.offers[name]; !ok {
		log.V(2).Infof("AgentState knows nothing of JobOffer(%s)", name)
		return
	}

	delete(self.offers, name)
}

func (self *AgentState) TrackBid(name string) {
	self.bids[name] = true
}

func (self *AgentState) HasBid(name string) bool {
	return self.bids[name]
}

func (self *AgentState) DropBid(name string) {
	delete(self.bids, name)
}

func globMatches(pattern, target string) bool {
	matched, err := path.Match(pattern, target)
	if err != nil {
		log.V(2).Infof("Received error while matching pattern '%s': %v", pattern, err)
	}
	return matched
}

func (self *AgentState) SetTargetState(jobName string, state job.JobState) {
	self.targetStates[jobName] = state
}

func (self *AgentState) DropTargetState(jobName string) {
	delete(self.targetStates, jobName)
}

func (self *AgentState) LaunchedJobs() []string {
	jobs := make([]string, 0)
	for j, ts := range self.targetStates {
		if ts == job.JobStateLaunched {
			jobs = append(jobs, j)
		}
	}
	return jobs
}

func (self *AgentState) ScheduledHere(jobName string) bool {
	ts := self.targetStates[jobName]
	return ts == job.JobStateLoaded || ts == job.JobStateLaunched
}

func (self *AgentState) ScheduledJobs() []string {
	jobs := make([]string, 0)
	for j, ts := range self.targetStates {
		if ts == job.JobStateLoaded || ts == job.JobStateLaunched {
			jobs = append(jobs, j)
		}
	}
	return jobs
}
