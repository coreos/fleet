// Copyright 2014 The fleet Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package agent

import (
	"fmt"
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/unit"
)

func fleetUnit(t *testing.T, opts ...string) unit.UnitFile {
	contents := "[X-Fleet]"
	for _, v := range opts {
		contents = fmt.Sprintf("%s\n%s", contents, v)
	}

	u, err := unit.NewUnitFile(contents)
	if u == nil || err != nil {
		t.Fatalf("Failed creating test unit: unit=%v, err=%v", u, err)
	}

	return *u
}

func TestHasConflicts(t *testing.T) {
	tests := []struct {
		cState   *AgentState
		job      *job.Job
		want     bool
		conflict string
	}{
		// empty current state causes no conflicts
		{
			cState: NewAgentState(&machine.MachineState{ID: "XXX"}),
			job:    &job.Job{Name: "foo.service", Unit: fleetUnit(t, "Conflicts=bar.service")},
			want:   false,
		},

		// existing Job has conflict with new Job
		{
			cState: &AgentState{
				MState: &machine.MachineState{ID: "XXX"},
				Units: map[string]*job.Unit{
					"bar.service": &job.Unit{
						Name: "bar.service",
						Unit: fleetUnit(t, "Conflicts=foo.service"),
					},
				},
			},
			job:      &job.Job{Name: "foo.service", Unit: unit.UnitFile{}},
			want:     true,
			conflict: "bar.service",
		},

		// new Job has conflict with existing job
		{
			cState: &AgentState{
				MState: &machine.MachineState{ID: "XXX"},
				Units: map[string]*job.Unit{
					"bar.service": &job.Unit{
						Name: "bar.service",
						Unit: unit.UnitFile{},
					},
				},
			},
			job:      &job.Job{Name: "foo.service", Unit: fleetUnit(t, "Conflicts=bar.service")},
			want:     true,
			conflict: "bar.service",
		},
	}

	for i, tt := range tests {
		got, conflict := tt.cState.HasConflict(tt.job.Name, tt.job.Conflicts())
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

func TestHasReplaces(t *testing.T) {
	tests := []struct {
		cState  *AgentState
		job     *job.Job
		want    bool
		replace string
	}{
		// empty current state causes no replaces
		{
			cState: NewAgentState(&machine.MachineState{ID: "XXX"}),
			job:    &job.Job{Name: "foo.service", Unit: fleetUnit(t, "Replaces=bar.service")},
			want:   false,
		},

		// existing Job has replace with new Job
		{
			cState: &AgentState{
				MState: &machine.MachineState{ID: "XXX"},
				Units: map[string]*job.Unit{
					"bar.service": &job.Unit{
						Name: "bar.service",
						Unit: fleetUnit(t, "Replaces=foo.service"),
					},
				},
			},
			job:     &job.Job{Name: "foo.service", Unit: unit.UnitFile{}},
			want:    true,
			replace: "bar.service",
		},

		// new Job has replace with existing job
		{
			cState: &AgentState{
				MState: &machine.MachineState{ID: "XXX"},
				Units: map[string]*job.Unit{
					"bar.service": &job.Unit{
						Name: "bar.service",
						Unit: unit.UnitFile{},
					},
				},
			},
			job:     &job.Job{Name: "foo.service", Unit: fleetUnit(t, "Replaces=bar.service")},
			want:    true,
			replace: "bar.service",
		},

		// both jobs have replace with each other: it should fail
		{
			cState: &AgentState{
				MState: &machine.MachineState{ID: "XXX"},
				Units: map[string]*job.Unit{
					"bar.service": &job.Unit{
						Name: "bar.service",
						Unit: fleetUnit(t, "Replaces=foo.service"),
					},
				},
			},
			job:     &job.Job{Name: "foo.service", Unit: fleetUnit(t, "Replaces=bar.service")},
			want:    false,
			replace: "bar.service",
		},
	}

	for i, tt := range tests {
		got, replace := tt.cState.hasReplace(tt.job.Name, tt.job.Replaces())
		if got != tt.want {
			var msg string
			if tt.want == true {
				msg = fmt.Sprintf("expected no replace, found replace with Job %q", replace)
			} else {
				msg = fmt.Sprintf("expected replace with Job %q, got none", replace)
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
