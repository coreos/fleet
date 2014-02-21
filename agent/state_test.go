package agent

import (
	"testing"
)

// Assert that an existing conflict is triggered against the potential job name
func TestHasConflictExistingMatch(t *testing.T) {
	state := NewState()
	state.TrackJobConflicts("a", []string{"b"})

	matched, name := state.HasConflict("b", []string{})
	if !matched || name != "a" {
		t.Errorf("Expected conflict with 'a'")
	}
}

// Assert that a potential conflict is triggered against the existing job name
func TestHasConflictPotentialMatch(t *testing.T) {
	state := NewState()
	state.TrackJobConflicts("a", []string{})

	matched, name := state.HasConflict("b", []string{"a"})
	if !matched || name != "a" {
		t.Errorf("Expected conflict with 'a'")
	}
}

// Assert that a existing jobs and potential jobs that do not conflict do not
// trigger a match
func TestHasConflictNoMatch(t *testing.T) {
	state := NewState()
	state.TrackJobConflicts("a", []string{"b"})

	matched, _ := state.HasConflict("c", []string{"d"})
	if matched {
		t.Errorf("Expected no match")
	}
}

// Assert that our glob-parser can handle relatively-complex matching
func TestHasConflictComplexGlob(t *testing.T) {
	state := NewState()
	state.TrackJobConflicts("a", []string{"*.[1-9].service"})

	matched, name := state.HasConflict("web.2.service", []string{})
	if !matched || name != "a" {
		t.Errorf("Expected conflict with 'a'")
	}

	matched, _ = state.HasConflict("app.99.service", []string{})
	if matched {
		t.Errorf("Expected no conflict")
	}
}

// Assert that a conflict is truly gone when DropJobConflicts is called
func TestHasConflictDropped(t *testing.T) {
	state := NewState()
	state.TrackJobConflicts("a", []string{"b"})

	matched, name := state.HasConflict("b", []string{})
	if !matched || name != "a" {
		t.Errorf("Expected conflict with 'a'")
	}

	state.DropJobConflicts("a")
	matched, _ = state.HasConflict("b", []string{})
	if matched {
		t.Errorf("Expected no conflict")
	}
}

// Assert that jobs and their peers are properly indexed
func TestGetJobsByPeer(t *testing.T) {
	state := NewState()
	state.TrackJobPeers("a", []string{"b", "c"})
	state.TrackJobPeers("d", []string{"c"})

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
	state := NewState()
	state.TrackJobPeers("a", []string{"b"})

	peers := state.GetJobsByPeer("c")
	if len(peers) != 0 {
		t.Fatalf("Unexpected index of job peers %v", peers)
	}
}

// Assert that peers indexes are properly cleared after
// calling DropPeersJob
func TestDropPeersJob(t *testing.T) {
	state := NewState()
	state.TrackJobPeers("a", []string{"b", "c"})
	state.TrackJobPeers("d", []string{"c"})
	state.DropPeersJob("a")

	peers := state.GetJobsByPeer("c")
	if len(peers) != 1 || peers[0] != "d" {
		t.Fatalf("Unexpected index of job peers %v", peers)
	}
}
