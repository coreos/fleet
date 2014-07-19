package agent

import "testing"

// Assert that jobs and their peers are properly indexed
func TestGetJobsByPeer(t *testing.T) {
	state := NewCache()

	j1 := newNamedTestJobWithXFleetValues(t, "a", `
X-ConditionMachineOf=b
X-ConditionMachineOf=c
`)
	state.TrackJob(j1)

	j2 := newNamedTestJobWithXFleetValues(t, "d", `[X-Fleet]
X-ConditionMachineOf=c
`)
	state.TrackJob(j2)

	peers := state.GetJobsByPeer("b")
	if len(peers) != 1 || peers[0] != "a" {
		t.Fatalf("Unexpected index of job peers %v", peers)
	}

	peers = state.GetJobsByPeer("c")
	if len(peers) != 2 || peers[0] != "a" || peers[1] != "d" {
		t.Fatalf("Unexpected index of job peers %v", peers)
	}
}

// Assert that no jobs are returned for unknown peers
func TestGetJobsByPeerUnknown(t *testing.T) {
	state := NewCache()

	j := newNamedTestJobWithXFleetValues(t, "a", `X-ConditionMachineOf=b`)
	state.TrackJob(j)

	peers := state.GetJobsByPeer("c")
	if len(peers) != 0 {
		t.Fatalf("Unexpected index of job peers %v", peers)
	}
}

// Assert that peers indexes are properly cleared after
// calling DropPeersJob
func TestDropPeersJob(t *testing.T) {
	state := NewCache()

	j1 := newNamedTestJobWithXFleetValues(t, "a", `[X-Fleet]
X-ConditionMachineOf=b
X-ConditionMachineOf=c
`)
	state.TrackJob(j1)

	j2 := newNamedTestJobWithXFleetValues(t, "d", `[X-Fleet]
X-ConditionMachineOf=c
`)
	state.TrackJob(j2)

	state.PurgeJob(j1.Name)

	peers := state.GetJobsByPeer("c")
	if len(peers) != 1 || peers[0] != "d" {
		t.Fatalf("Unexpected index of job peers %v", peers)
	}
}
