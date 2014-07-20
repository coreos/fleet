package agent

import (
	"fmt"
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/unit"
)

func TestHasConflicts(t *testing.T) {
	tests := []struct {
		cState   *agentState
		job      *job.Job
		want     bool
		conflict string
	}{
		// empty current state causes no conflicts
		{
			cState: newAgentState(),
			job:    &job.Job{Name: "foo.service", Unit: fleetUnit(t, "X-Conflicts=bar.service")},
			want:   false,
		},

		// existing Job has conflict with new Job
		{
			cState: &agentState{
				jobs: map[string]*job.Job{
					"bar.service": &job.Job{
						Name: "bar.service",
						Unit: fleetUnit(t, "X-Conflicts=foo.service"),
					},
				},
			},
			job:      &job.Job{Name: "foo.service", Unit: unit.Unit{}},
			want:     true,
			conflict: "bar.service",
		},

		// new Job has conflict with existing job
		{
			cState: &agentState{
				jobs: map[string]*job.Job{
					"bar.service": &job.Job{
						Name: "bar.service",
						Unit: unit.Unit{},
					},
				},
			},
			job:      &job.Job{Name: "foo.service", Unit: fleetUnit(t, "X-Conflicts=bar.service")},
			want:     true,
			conflict: "bar.service",
		},
	}

	for i, tt := range tests {
		got, conflict := tt.cState.hasConflict(tt.job.Name, tt.job.Conflicts())
		if got != tt.want {
			var msg string
			if tt.want == true {
				msg = fmt.Sprintf("expected no conflict, found conflict with Job %q", conflict)
			} else {
				msg = fmt.Sprintf("expected conflict with Job %q, got none", conflict)
			}
			t.Errorf("case %d: %s", i, msg)
		}
	}
}

func TestGlobMatches(t *testing.T) {
	tests := []struct {
		pattern  string
		argument string
		want     bool
	}{
		{"*", "foo.service", true},
		{"foo.*", "foo.socket", true},
		{"foo@*.service", "foo@12.service", true},
		{"foo@[abc].service", "foo@a.service", true},
		{"foo@?.service", "foo@1.service", true},

		{"foo.service", "bar.service", false},
		{"foo@[abc].service", "foo@d.service", false},
	}

	for i, tt := range tests {
		got := globMatches(tt.pattern, tt.argument)
		if got != tt.want {
			t.Errorf("case %d: pattern=%q argument=%q want=%t got=%t", i, tt.pattern, tt.argument, tt.want, got)
		}
	}
}
