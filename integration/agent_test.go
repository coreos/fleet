package integration

import (
	"testing"

	"github.com/coreos/fleet/agent"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/unit"
)

func TestIntegrationAgentBiddingConflicts(t *testing.T) {
	fm := machine.FakeMachine{machine.MachineState{ID: "XXX"}}
	fum := unit.NewFakeUnitManager()
	freg := registry.NewFakeRegistry()
	agent, err := agent.New(fum, freg, &fm, agent.DefaultTTL, nil)
	if err != nil {
		t.Fatalf("Failed creating new Agent: %v", err)
	}

	content := `[X-Fleet]
X-Conflicts=*
`

	tests := []struct {
		// Name of Job to create and offer
		jobName string

		// Agent expected to bid on the offered Job or not
		expectBid bool

		// Function that should be called after Job is created, offered
		// and any assertions are made
		cb func(string)
	}{
		{"j1.service", true, nil},
		{"j2.service", true, agent.JobScheduledLocally},
		{"j3.service", false, agent.JobScheduledElsewhere},
	}

	for i, tt := range tests {
		j := job.NewJob(tt.jobName, *unit.NewUnit(content))
		freg.CreateJob(j)

		jo := job.NewOfferFromJob(*j, []string{})
		agent.MaybeBid(*jo)

		bids, err := freg.Bids(jo)
		if err != nil {
			t.Errorf("Received error from Registry.Bids: %v", err)
		}

		didBid := len(bids) > 0 && bids[0].JobName == tt.jobName
		if didBid != tt.expectBid {
			t.Errorf("test %d, bid = %v, want %v", i, didBid, tt.expectBid)
		}

		if tt.cb != nil {
			tt.cb(tt.jobName)
		}
	}
}
