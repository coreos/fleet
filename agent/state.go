package agent

type AgentState struct {
	// map of Peer dependencies to unresolved JobOffers
	peers map[string][]string
}

func NewState() *AgentState {
	peers := make(map[string][]string, 0)
	return &AgentState{peers}
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
