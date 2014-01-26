package agent

import (
	"sync"

	log "github.com/golang/glog"

	"github.com/coreos/coreinit/job"
)

type AgentState struct {
	// used to lock the datastructure for multi-goroutine safety
	mutex sync.Mutex

	// unresolved job offers
	offers map[string]job.JobOffer

	// job names for which a bid has been submitted
	bids []string

	// reverse index of peers that would cause a reassesment of a JobOffer this
	// Agent could not have bid on previously
	// i.e. {"hello.service": ["howareyou.service", "goodbye.service"]}
	peers map[string][]string
}

func NewState() *AgentState {
	return &AgentState{offers: make(map[string]job.JobOffer), bids: make([]string, 0), peers: make(map[string][]string)}
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
			self.peers[peer] = append(self.peers[peer][0:idx-i], self.peers[peer][idx-i:]...)
		}
	}
}

func (self *AgentState) TrackOffer(offer job.JobOffer) {
	if _, ok := self.GetOffer(offer.Job.Name); ok {
		log.V(2).Infof("AgentState already knows about JobOffer(%s)", offer.Job.Name)
		return
	}

	self.offers[offer.Job.Name] = offer
}

func (self *AgentState) GetBadeOffers() []job.JobOffer {
	offers := make([]job.JobOffer, 0)
	for _, offer := range self.offers {
		if self.HasBid(offer.Job.Name) {
			offers = append(offers, offer)
		}
	}
	return offers
}

func (self *AgentState) GetUnbadeOffers() []job.JobOffer {
	offers := make([]job.JobOffer, 0)
	for _, offer := range self.offers {
		if !self.HasBid(offer.Job.Name) {
			offers = append(offers, offer)
		}
	}
	return offers
}

func (self *AgentState) GetOffer(name string) (job.JobOffer, bool) {
	offer, ok := self.offers[name]
	return offer, ok
}

func (self *AgentState) DropOffer(name string) {
	if _, ok := self.offers[name]; !ok {
		log.V(2).Infof("AgentState knows nothing of JobOffer(%s)", name)
		return
	}

	delete(self.offers, name)
}

func (self *AgentState) TrackBid(name string) {
	self.bids = append(self.bids, name)
}

func (self *AgentState) HasBid(name string) bool {
	for _, val := range self.bids {
		if val == name {
			return true
		}
	}
	return false
}

func (self *AgentState) DropBid(name string) {
	for idx, val := range self.bids {
		if val == name {
			self.bids = append(self.bids[0:idx], self.bids[idx+1:]...)
			return
		}
	}
}
