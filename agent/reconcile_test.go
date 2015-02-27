// Copyright 2014 CoreOS, Inc.
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
	"reflect"
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/unit"
)

// Represents the hash of e.g. an empty unit
// echo -n "" | sha1sum
const emptyStringHash = "da39a3ee5e6b4b0d3255bfef95601890afd80709"

var (
	jsInactive = job.JobStateInactive
	jsLoaded   = job.JobStateLoaded
	jsLaunched = job.JobStateLaunched
)

func makeAgentWithMetadata(md map[string]string) *Agent {
	return &Agent{
		Machine: &machine.FakeMachine{
			MachineState: machine.MachineState{
				ID:       "this_machine",
				Metadata: md,
			},
		},
	}
}

func newUF(t *testing.T, contents string) unit.UnitFile {
	uf, err := unit.NewUnitFile(contents)
	if err != nil {
		t.Fatalf("error creating new unit file from %v: %v", contents, err)
	}
	return *uf
}

func TestDesiredAgentState(t *testing.T) {
	testCases := []struct {
		metadata map[string]string
		regJobs  []job.Job
		asUnits  map[string]*job.Unit
	}{
		// No units in the registry = nothing to do
		{
			nil,
			nil,
			map[string]*job.Unit{},
		},
		// Single Unit scheduled to this machine. Easy.
		{
			nil,
			[]job.Job{
				job.Job{
					Name:            "foo.service",
					Unit:            newUF(t, "blah"),
					TargetMachineID: "this_machine",
				},
			},
			map[string]*job.Unit{
				"foo.service": &job.Unit{
					Name: "foo.service",
					Unit: newUF(t, "blah"),
				},
			},
		},
		// Unit scheduled nowhere - ignore it
		{
			nil,
			[]job.Job{
				job.Job{
					Name: "foo.service",
					Unit: newUF(t, "blah"),
				},
			},
			map[string]*job.Unit{},
		},
		// Unit scheduled somewhere else - ignore it
		{
			nil,
			[]job.Job{
				job.Job{
					Name:            "foo.service",
					Unit:            newUF(t, "blah"),
					TargetMachineID: "elsewhere",
				},
			},
			map[string]*job.Unit{},
		},
		// Global Unit with no metadata? No problem
		{
			nil,
			[]job.Job{
				job.Job{
					Name: "global.service",
					Unit: newUF(t, "[X-Fleet]\nGlobal=true"),
				},
			},
			map[string]*job.Unit{
				"global.service": &job.Unit{
					Name: "global.service",
					Unit: newUF(t, "[X-Fleet]\nGlobal=true"),
				},
			},
		},
		// Global Unit with metadata we have? Great
		{
			map[string]string{"dog": "woof"},
			[]job.Job{
				job.Job{
					Name: "global.mount",
					Unit: newUF(t, `
[X-Fleet]
Global=true
MachineMetadata=dog=woof`),
				},
			},
			map[string]*job.Unit{
				"global.mount": &job.Unit{
					Name: "global.mount",
					Unit: newUF(t, `
[X-Fleet]
Global=true
MachineMetadata=dog=woof`),
				},
			},
		},
		// Global Unit with metadata we don't? Uhoh
		{
			nil,
			[]job.Job{
				job.Job{
					Name: "global.mount",
					Unit: newUF(t, `
[X-Fleet]
Global=true
MachineMetadata=dog=woof`),
				},
			},
			map[string]*job.Unit{},
		},
		{
			map[string]string{"cat": "miaow"},
			[]job.Job{
				job.Job{
					Name: "global.mount",
					Unit: newUF(t, `
[X-Fleet]
Global=true
MachineMetadata=dog=woof`),
				},
			},
			map[string]*job.Unit{},
		},
		// Mix it up a bit!
		{
			nil,
			[]job.Job{
				job.Job{
					Name:            "foo.service",
					Unit:            newUF(t, "blah"),
					TargetMachineID: "this_machine",
				},
				job.Job{
					Name: "bar.service",
					Unit: newUF(t, "blah"),
				},
				job.Job{
					Name: "global.service",
					Unit: newUF(t, "[X-Fleet]\nGlobal=true"),
				},
				job.Job{
					Name: "global.mount",
					Unit: newUF(t, `
[X-Fleet]
Global=true
MachineMetadata=dog=woof`),
				},
			},
			map[string]*job.Unit{
				"foo.service": &job.Unit{
					Name: "foo.service",
					Unit: newUF(t, "blah"),
				},
				"global.service": &job.Unit{
					Name: "global.service",
					Unit: newUF(t, "[X-Fleet]\nGlobal=true"),
				},
			},
		},
		{
			map[string]string{"dog": "woof"},
			[]job.Job{
				job.Job{
					Name:            "foo.service",
					Unit:            newUF(t, "blah"),
					TargetMachineID: "this_machine",
				},
				job.Job{
					Name: "bar.service",
					Unit: newUF(t, "blah"),
				},
				job.Job{
					Name: "global.service",
					Unit: newUF(t, "[X-Fleet]\nGlobal=true"),
				},
				job.Job{
					Name: "global.mount",
					Unit: newUF(t, `
[X-Fleet]
Global=true
MachineMetadata=dog=woof`),
				},
			},
			map[string]*job.Unit{
				"foo.service": &job.Unit{
					Name: "foo.service",
					Unit: newUF(t, "blah"),
				},
				"global.service": &job.Unit{
					Name: "global.service",
					Unit: newUF(t, "[X-Fleet]\nGlobal=true"),
				},
				"global.mount": &job.Unit{
					Name: "global.mount",
					Unit: newUF(t, `
[X-Fleet]
Global=true
MachineMetadata=dog=woof`),
				},
			},
		},
	}

	for i, tt := range testCases {
		reg := registry.NewFakeRegistry()
		reg.SetJobs(tt.regJobs)
		a := makeAgentWithMetadata(tt.metadata)
		as, err := desiredAgentState(a, reg)
		if err != nil {
			t.Errorf("case %d: unexpected error: %v", i, err)
		} else if !reflect.DeepEqual(as.Units, tt.asUnits) {
			t.Errorf("case %d: AgentState differs to expected", i)
			t.Logf("found:\n")
			for _, u := range as.Units {
				t.Logf("  %#v", u)
			}
			t.Logf("expected:\n")
			for _, u := range tt.asUnits {
				t.Logf("  %#v", u)
			}
		}
	}
}

func TestAbleToRun(t *testing.T) {
	tests := []struct {
		dState *AgentState
		job    *job.Job
		want   bool
	}{
		// nothing to worry about
		{
			dState: NewAgentState(&machine.MachineState{ID: "123"}),
			job:    &job.Job{Name: "easy-street.service", Unit: unit.UnitFile{}},
			want:   true,
		},

		// match MachineID
		{
			dState: NewAgentState(&machine.MachineState{ID: "XYZ"}),
			job:    newTestJobWithXFleetValues(t, "MachineID=XYZ"),
			want:   true,
		},

		// mismatch MachineID
		{
			dState: NewAgentState(&machine.MachineState{ID: "123"}),
			job:    newTestJobWithXFleetValues(t, "MachineID=XYZ"),
			want:   false,
		},

		// match MachineMetadata
		{
			dState: NewAgentState(&machine.MachineState{ID: "123", Metadata: map[string]string{"region": "us-west"}}),
			job:    newTestJobWithXFleetValues(t, "MachineMetadata=region=us-west"),
			want:   true,
		},

		// Machine metadata ignored when no MachineMetadata in Job
		{
			dState: NewAgentState(&machine.MachineState{ID: "123", Metadata: map[string]string{"region": "us-west"}}),
			job:    &job.Job{Name: "easy-street.service", Unit: unit.UnitFile{}},
			want:   true,
		},

		// mismatch MachineMetadata
		{
			dState: NewAgentState(&machine.MachineState{ID: "123", Metadata: map[string]string{"region": "us-west"}}),
			job:    newTestJobWithXFleetValues(t, "MachineMetadata=region=us-east"),
			want:   false,
		},

		// peer scheduled locally
		{
			dState: &AgentState{
				MState: &machine.MachineState{ID: "123"},
				Units: map[string]*job.Unit{
					"pong.service": &job.Unit{Name: "pong.service"},
				},
			},
			job:  newTestJobWithXFleetValues(t, "MachineOf=pong.service"),
			want: true,
		},

		// multiple peers scheduled locally
		{
			dState: &AgentState{
				MState: &machine.MachineState{ID: "123"},
				Units: map[string]*job.Unit{
					"ping.service": &job.Unit{Name: "ping.service"},
					"pong.service": &job.Unit{Name: "pong.service"},
				},
			},
			job:  newTestJobWithXFleetValues(t, "MachineOf=pong.service\nMachineOf=ping.service"),
			want: true,
		},

		// peer not scheduled locally
		{
			dState: NewAgentState(&machine.MachineState{ID: "123"}),
			job:    newTestJobWithXFleetValues(t, "MachineOf=ping.service"),
			want:   false,
		},

		// one of multiple peers not scheduled locally
		{
			dState: &AgentState{
				MState: &machine.MachineState{ID: "123"},
				Units: map[string]*job.Unit{
					"ping.service": &job.Unit{Name: "ping.service"},
				},
			},
			job:  newTestJobWithXFleetValues(t, "MachineOf=pong.service\nMachineOf=ping.service"),
			want: false,
		},

		// no conflicts found
		{
			dState: &AgentState{
				MState: &machine.MachineState{ID: "123"},
				Units: map[string]*job.Unit{
					"ping.service": &job.Unit{Name: "ping.service"},
				},
			},
			job:  newTestJobWithXFleetValues(t, "Conflicts=pong.service"),
			want: true,
		},

		// conflicts found
		{
			dState: &AgentState{
				MState: &machine.MachineState{ID: "123"},
				Units: map[string]*job.Unit{
					"ping.service": &job.Unit{Name: "ping.service"},
				},
			},
			job:  newTestJobWithXFleetValues(t, "Conflicts=ping.service"),
			want: false,
		},
	}

	for i, tt := range tests {
		got, _ := tt.dState.AbleToRun(tt.job)
		if got != tt.want {
			t.Errorf("case %d: expected %t, got %t", i, tt.want, got)
		}
	}
}

func TestCalculateTasksForJob(t *testing.T) {
	tests := []struct {
		dState *AgentState
		cState unitStates
		uName  string

		want []task
	}{

		// nil agent state objects should result in no tasks
		{
			dState: nil,
			cState: nil,
			uName:  "foo.service",
			want:   nil,
		},

		// nil job should result in no tasks
		{
			dState: NewAgentState(&machine.MachineState{ID: "XXX"}),
			cState: unitStates{},
			uName:  "foo.service",
			want:   nil,
		},

		// no work needs to be done when current state == desired state
		{
			dState: &AgentState{
				MState: &machine.MachineState{ID: "XXX"},
				Units: map[string]*job.Unit{
					"foo.service": &job.Unit{TargetState: jsLoaded},
				},
			},
			cState: unitStates{
				"foo.service": unitState{
					state: jsLoaded,
					hash:  emptyStringHash,
				},
			},
			uName: "foo.service",
			want:  nil,
		},

		// no work needs to be done when current state == desired state
		{
			dState: &AgentState{
				MState: &machine.MachineState{ID: "XXX"},
				Units: map[string]*job.Unit{
					"foo.service": &job.Unit{TargetState: jsLaunched},
				},
			},
			cState: unitStates{
				"foo.service": unitState{
					state: jsLaunched,
					hash:  emptyStringHash,
				},
			},
			uName: "foo.service",
			want:  nil,
		},

		// when current state == desired state but hash differs, unit should be unloaded and then reloaded
		{
			dState: &AgentState{
				MState: &machine.MachineState{ID: "XXX"},
				Units: map[string]*job.Unit{
					"foo.service": &job.Unit{TargetState: jsLaunched},
				},
			},
			cState: unitStates{
				"foo.service": unitState{
					state: jsLaunched,
					hash:  "abcdefg",
				},
			},
			uName: "foo.service",
			want: []task{
				task{
					typ:    taskTypeUnloadUnit,
					reason: taskReasonLoadedButHashDiffers,
					unit: &job.Unit{
						Name: "foo.service",
						Unit: unit.UnitFile{},
					},
				},
				task{
					typ:    taskTypeLoadUnit,
					reason: taskReasonScheduledButUnloaded,
					unit: &job.Unit{
						Name: "foo.service",
						Unit: unit.UnitFile{},
					},
				},
				task{
					typ:    taskTypeStartUnit,
					reason: taskReasonLoadedDesiredStateLaunched,
					unit: &job.Unit{
						Name: "foo.service",
						Unit: unit.UnitFile{},
					},
				},
			},
		},

		// when current state != desired state and hash differs, unit should be unloaded and then reloaded
		{
			dState: &AgentState{
				MState: &machine.MachineState{ID: "XXX"},
				Units: map[string]*job.Unit{
					"foo.service": &job.Unit{TargetState: jsLoaded},
				},
			},
			cState: unitStates{
				"foo.service": unitState{
					state: jsLaunched,
					hash:  "abcdefg",
				},
			},
			uName: "foo.service",
			want: []task{
				task{
					typ:    taskTypeUnloadUnit,
					reason: taskReasonLoadedButHashDiffers,
					unit: &job.Unit{
						Name: "foo.service",
						Unit: unit.UnitFile{},
					},
				},
				task{
					typ:    taskTypeLoadUnit,
					reason: taskReasonScheduledButUnloaded,
					unit: &job.Unit{
						Name: "foo.service",
						Unit: unit.UnitFile{},
					},
				},
			},
		},

		// when current state != desired state and hash differs, unit should be unloaded and then reloaded
		{
			dState: &AgentState{
				MState: &machine.MachineState{ID: "XXX"},
				Units: map[string]*job.Unit{
					"foo.service": &job.Unit{TargetState: jsLaunched},
				},
			},
			cState: unitStates{
				"foo.service": unitState{
					state: jsLoaded,
					hash:  "abcdefg",
				},
			},
			uName: "foo.service",
			want: []task{
				task{
					typ:    taskTypeUnloadUnit,
					reason: taskReasonLoadedButHashDiffers,
					unit: &job.Unit{
						Name: "foo.service",
						Unit: unit.UnitFile{},
					},
				},
				task{
					typ:    taskTypeLoadUnit,
					reason: taskReasonScheduledButUnloaded,
					unit: &job.Unit{
						Name: "foo.service",
						Unit: unit.UnitFile{},
					},
				},
				task{
					typ:    taskTypeStartUnit,
					reason: taskReasonLoadedDesiredStateLaunched,
					unit: &job.Unit{
						Name: "foo.service",
						Unit: unit.UnitFile{},
					},
				},
			},
		},

		// no work needs to be done when desired state == inactive and current state == nil
		{
			dState: &AgentState{
				MState: &machine.MachineState{ID: "XXX"},
				Units: map[string]*job.Unit{
					"foo.service": &job.Unit{TargetState: jsInactive},
				},
			},
			cState: unitStates{},
			uName:  "foo.service",
			want:   nil,
		},

		// load jobs that have a loaded desired state
		{
			dState: &AgentState{
				MState: &machine.MachineState{ID: "XXX"},
				Units: map[string]*job.Unit{
					"foo.service": &job.Unit{TargetState: jsLoaded},
				},
			},
			cState: unitStates{},
			uName:  "foo.service",
			want: []task{
				task{
					typ:    taskTypeLoadUnit,
					reason: taskReasonScheduledButUnloaded,
					unit: &job.Unit{
						Name: "foo.service",
						Unit: unit.UnitFile{},
					},
				},
			},
		},

		// load + launch jobs that have a launched desired state
		{
			dState: &AgentState{
				MState: &machine.MachineState{ID: "XXX"},
				Units: map[string]*job.Unit{
					"foo.service": &job.Unit{TargetState: jsLaunched},
				},
			},
			cState: unitStates{},
			uName:  "foo.service",
			want: []task{
				task{
					typ:    taskTypeLoadUnit,
					reason: taskReasonScheduledButUnloaded,
					unit: &job.Unit{
						Name: "foo.service",
					},
				},
				task{
					typ:    taskTypeStartUnit,
					reason: taskReasonLoadedDesiredStateLaunched,
					unit: &job.Unit{
						Name: "foo.service",
					},
				},
			},
		},

		// unload jobs that are no longer scheduled locally
		{
			dState: NewAgentState(&machine.MachineState{ID: "XXX"}),
			cState: unitStates{
				"foo.service": unitState{
					state: jsLoaded,
					hash:  emptyStringHash,
				},
			},
			uName: "foo.service",
			want: []task{
				task{
					typ:    taskTypeUnloadUnit,
					reason: taskReasonLoadedButNotScheduled,
					unit: &job.Unit{
						Name: "foo.service",
					},
				},
			},
		},

		// unload jobs that are no longer scheduled locally
		{
			dState: NewAgentState(&machine.MachineState{ID: "XXX"}),
			cState: unitStates{
				"foo.service": unitState{
					state: jsLaunched,
					hash:  emptyStringHash,
				},
			},
			uName: "foo.service",
			want: []task{
				task{
					typ:    taskTypeUnloadUnit,
					reason: taskReasonLoadedButNotScheduled,
					unit: &job.Unit{
						Name: "foo.service",
					},
				},
			},
		},

		// unload jobs that have an inactive target state
		{
			dState: &AgentState{
				MState: &machine.MachineState{ID: "XXX"},
				Units: map[string]*job.Unit{
					"foo.service": &job.Unit{
						TargetState: jsInactive,
					},
				},
			},
			cState: unitStates{
				"foo.service": unitState{
					state: jsLoaded,
					hash:  emptyStringHash,
				},
			},
			uName: "foo.service",
			want: []task{
				task{
					typ:    taskTypeUnloadUnit,
					reason: taskReasonLoadedButNotScheduled,
					unit: &job.Unit{
						Name: "foo.service",
					},
				},
			},
		},
	}

	for i, tt := range tests {
		ar := NewReconciler(registry.NewFakeRegistry(), nil)
		got := ar.calculateTasksForUnit(tt.dState, tt.cState, tt.uName)
		if !reflect.DeepEqual(tt.want, got) {
			t.Errorf("case %d: calculated incorrect tasks\nexpected=%#v\nreceived=%#v\n", i, tt.want, got)
		}
	}
}
