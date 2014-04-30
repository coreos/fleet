package agent

import (
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/unit"
)

// Assert that jobs and their peers are properly indexed
func TestGetJobsByPeer(t *testing.T) {
	state := NewState()

	u1 := unit.NewSystemdUnitFile(`[X-Fleet]
X-ConditionMachineOf=b
X-ConditionMachineOf=c
`)
	p1 := job.NewJobPayload("a", *u1)
	j1 := job.NewJob("a", *p1)
	state.TrackJob(j1)

	u2 := unit.NewSystemdUnitFile(`[X-Fleet]
X-ConditionMachineOf=c
`)
	p2 := job.NewJobPayload("d", *u2)
	j2 := job.NewJob("d", *p2)
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
	u := unit.NewSystemdUnitFile(`[X-Fleet]
X-ConditionMachineOf=b
`)
	p := job.NewJobPayload("a", *u)
	j := job.NewJob("a", *p)

	state := NewState()
	state.TrackJob(j)

	peers := state.GetJobsByPeer("c")
	if len(peers) != 0 {
		t.Fatalf("Unexpected index of job peers %v", peers)
	}
}

// Assert that peers indexes are properly cleared after
// calling DropPeersJob
func TestDropPeersJob(t *testing.T) {
	state := NewState()

	u1 := unit.NewSystemdUnitFile(`[X-Fleet]
X-ConditionMachineOf=b
X-ConditionMachineOf=c
`)
	p1 := job.NewJobPayload("a", *u1)
	j1 := job.NewJob("a", *p1)
	state.TrackJob(j1)

	u2 := unit.NewSystemdUnitFile(`[X-Fleet]
X-ConditionMachineOf=c
`)
	p2 := job.NewJobPayload("d", *u2)
	j2 := job.NewJob("d", *p2)
	state.TrackJob(j2)

	state.PurgeJob(j1.Name)

	peers := state.GetJobsByPeer("c")
	if len(peers) != 1 || peers[0] != "d" {
		t.Fatalf("Unexpected index of job peers %v", peers)
	}
}
