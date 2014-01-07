package agent

import (
	"sync"

	log "github.com/golang/glog"
)

// AgentState tracks two things:
// 1. Jobs that have not yet been scheduled to the cluster and could
//    be fulfilled by this machine if a peer were to be scheduled here.
// 2. Pending JobBids that could cause conflicts with other JobBids.

type AgentState struct {
	// map of Peer dependencies to unresolved JobOffers
	peers map[string][]string

	// map of conflicting string to Job
	// i.e. {"hello": "hello.service"}
	conflicts map[string]string

	mutex sync.Mutex
}

func NewState() *AgentState {
	return &AgentState{peers: make(map[string][]string), conflicts: make(map[string]string)}
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

func (self *AgentState) TrackPeer(job string, peer string) {
	if _, ok := self.peers[job]; !ok {
		self.peers[job] = make([]string, 0)
	}
	self.peers[job] = append(self.peers[job], peer)
}

func (self *AgentState) GetPeers(job string) []string {
	peers, ok := self.peers[job]
	if ok {
		return peers
	} else {
		return make([]string, 0)
	}
}

func (self *AgentState) TrackConflict(conflict string, job string) {
	self.conflicts[conflict] = job
}

func (self *AgentState) GetConflict(conflict string) (string, bool) {
	j, ok := self.conflicts[conflict]
	return j, ok
}

func (self *AgentState) RemoveConflictsByJob(match string) {
	var keys []string

	for conflict, j := range self.conflicts {
		if j == match {
			keys = append(keys, conflict)
		}
	}

	for _, key := range keys {
		delete(self.conflicts, key)
	}
}
