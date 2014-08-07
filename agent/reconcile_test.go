package agent

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/unit"
)

var (
	jsInactive = job.JobStateInactive
	jsLoaded   = job.JobStateLoaded
	jsLaunched = job.JobStateLaunched
)

func fleetUnit(t *testing.T, opts ...string) unit.Unit {
	contents := "[X-Fleet]"
	for _, v := range opts {
		contents = fmt.Sprintf("%s\n%s", contents, v)
	}

	u, err := unit.NewUnit(contents)
	if u == nil || err != nil {
		t.Fatalf("Failed creating test unit: unit=%v, err=%v", u, err)
	}

	return *u
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
			job:    &job.Job{Name: "easy-street.service", Unit: unit.Unit{}},
			want:   true,
		},

		// match X-ConditionMachineID
		{
			dState: NewAgentState(&machine.MachineState{ID: "XYZ"}),
			job:    newTestJobWithXFleetValues(t, "X-ConditionMachineID=XYZ"),
			want:   true,
		},

		// mismatch X-ConditionMachineID
		{
			dState: NewAgentState(&machine.MachineState{ID: "123"}),
			job:    newTestJobWithXFleetValues(t, "X-ConditionMachineID=XYZ"),
			want:   false,
		},

		// match X-ConditionMachineMetadata
		{
			dState: NewAgentState(&machine.MachineState{ID: "123", Metadata: map[string]string{"region": "us-west"}}),
			job:    newTestJobWithXFleetValues(t, "X-ConditionMachineMetadata=region=us-west"),
			want:   true,
		},

		// Machine metadata ignored when no X-ConditionMachineMetadata in Job
		{
			dState: NewAgentState(&machine.MachineState{ID: "123", Metadata: map[string]string{"region": "us-west"}}),
			job:    &job.Job{Name: "easy-street.service", Unit: unit.Unit{}},
			want:   true,
		},

		// mismatch X-ConditionMachineMetadata
		{
			dState: NewAgentState(&machine.MachineState{ID: "123", Metadata: map[string]string{"region": "us-west"}}),
			job:    newTestJobWithXFleetValues(t, "X-ConditionMachineMetadata=region=us-east"),
			want:   false,
		},

		// peer scheduled locally
		{
			dState: &AgentState{
				MState: &machine.MachineState{ID: "123"},
				Jobs: map[string]*job.Job{
					"pong.service": &job.Job{Name: "pong.service"},
				},
			},
			job:  newTestJobWithXFleetValues(t, "X-ConditionMachineOf=pong.service"),
			want: true,
		},

		// multiple peers scheduled locally
		{
			dState: &AgentState{
				MState: &machine.MachineState{ID: "123"},
				Jobs: map[string]*job.Job{
					"ping.service": &job.Job{Name: "ping.service"},
					"pong.service": &job.Job{Name: "pong.service"},
				},
			},
			job:  newTestJobWithXFleetValues(t, "X-ConditionMachineOf=pong.service\nX-ConditionMachineOf=ping.service"),
			want: true,
		},

		// peer not scheduled locally
		{
			dState: NewAgentState(&machine.MachineState{ID: "123"}),
			job:    newTestJobWithXFleetValues(t, "X-ConditionMachineOf=ping.service"),
			want:   false,
		},

		// one of multiple peers not scheduled locally
		{
			dState: &AgentState{
				MState: &machine.MachineState{ID: "123"},
				Jobs: map[string]*job.Job{
					"ping.service": &job.Job{Name: "ping.service"},
				},
			},
			job:  newTestJobWithXFleetValues(t, "X-ConditionMachineOf=pong.service\nX-ConditionMachineOf=ping.service"),
			want: false,
		},

		// no conflicts found
		{
			dState: &AgentState{
				MState: &machine.MachineState{ID: "123"},
				Jobs: map[string]*job.Job{
					"ping.service": &job.Job{Name: "ping.service"},
				},
			},
			job:  newTestJobWithXFleetValues(t, "X-Conflicts=pong.service"),
			want: true,
		},

		// conflicts found
		{
			dState: &AgentState{
				MState: &machine.MachineState{ID: "123"},
				Jobs: map[string]*job.Job{
					"ping.service": &job.Job{Name: "ping.service"},
				},
			},
			job:  newTestJobWithXFleetValues(t, "X-Conflicts=ping.service"),
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
		cState *AgentState
		jName  string

		tasks []task
	}{

		// nil agent state objects should result in no tasks
		{
			dState: nil,
			cState: nil,
			jName:  "foo.service",
			tasks:  []task{},
		},

		// nil job should result in no tasks
		{
			dState: NewAgentState(&machine.MachineState{ID: "XXX"}),
			cState: NewAgentState(&machine.MachineState{ID: "XXX"}),
			jName:  "foo.service",
			tasks:  []task{},
		},

		// no work needs to be done when target state == desired state
		{
			dState: &AgentState{
				MState: &machine.MachineState{ID: "XXX"},
				Jobs: map[string]*job.Job{
					"foo.service": &job.Job{TargetState: jsLoaded},
				},
			},
			cState: &AgentState{
				MState: &machine.MachineState{ID: "XXX"},
				Jobs: map[string]*job.Job{
					"foo.service": &job.Job{State: &jsLoaded},
				},
			},
			jName: "foo.service",
			tasks: []task{},
		},

		// no work needs to be done when target state == desired state
		{
			dState: &AgentState{
				MState: &machine.MachineState{ID: "XXX"},
				Jobs: map[string]*job.Job{
					"foo.service": &job.Job{TargetState: jsLaunched},
				},
			},
			cState: &AgentState{
				MState: &machine.MachineState{ID: "XXX"},
				Jobs: map[string]*job.Job{
					"foo.service": &job.Job{State: &jsLaunched},
				},
			},
			jName: "foo.service",
			tasks: []task{},
		},

		// load jobs that have a loaded desired state
		{
			dState: &AgentState{
				MState: &machine.MachineState{ID: "XXX"},
				Jobs: map[string]*job.Job{
					"foo.service": &job.Job{TargetState: jsLoaded},
				},
			},
			cState: NewAgentState(&machine.MachineState{ID: "XXX"}),
			jName:  "foo.service",
			tasks: []task{
				task{
					Type:   taskTypeLoadJob,
					Job:    &job.Job{TargetState: jsLoaded},
					Reason: taskReasonScheduledButUnloaded,
				},
			},
		},

		// load jobs that have a launched desired state
		{
			dState: &AgentState{
				MState: &machine.MachineState{ID: "XXX"},
				Jobs: map[string]*job.Job{
					"foo.service": &job.Job{TargetState: jsLaunched},
				},
			},
			cState: NewAgentState(&machine.MachineState{ID: "XXX"}),
			jName:  "foo.service",
			tasks: []task{
				task{
					Type:   taskTypeLoadJob,
					Job:    &job.Job{TargetState: jsLaunched},
					Reason: taskReasonScheduledButUnloaded,
				},
			},
		},

		// unload jobs that are no longer scheduled locally
		{
			dState: NewAgentState(&machine.MachineState{ID: "XXX"}),
			cState: &AgentState{
				MState: &machine.MachineState{ID: "XXX"},
				Jobs: map[string]*job.Job{
					"foo.service": &job.Job{State: &jsLoaded},
				},
			},
			jName: "foo.service",
			tasks: []task{
				task{
					Type:   taskTypeUnloadJob,
					Job:    &job.Job{State: &jsLoaded},
					Reason: taskReasonLoadedButNotScheduled,
				},
			},
		},

		// unload jobs that are no longer scheduled locally
		{
			dState: NewAgentState(&machine.MachineState{ID: "XXX"}),
			cState: &AgentState{
				MState: &machine.MachineState{ID: "XXX"},
				Jobs: map[string]*job.Job{
					"foo.service": &job.Job{State: &jsLaunched},
				},
			},
			jName: "foo.service",
			tasks: []task{
				task{
					Type:   taskTypeUnloadJob,
					Job:    &job.Job{State: &jsLaunched},
					Reason: taskReasonLoadedButNotScheduled,
				},
			},
		},

		// unload jobs that have an inactive target state
		{
			dState: &AgentState{
				MState: &machine.MachineState{ID: "XXX"},
				Jobs: map[string]*job.Job{
					"foo.service": &job.Job{
						TargetState: jsInactive,
					},
				},
			},
			cState: &AgentState{
				MState: &machine.MachineState{ID: "XXX"},
				Jobs: map[string]*job.Job{
					"foo.service": &job.Job{State: &jsLoaded},
				},
			},
			jName: "foo.service",
			tasks: []task{
				task{
					Type:   taskTypeUnloadJob,
					Job:    &job.Job{State: &jsLoaded},
					Reason: taskReasonLoadedButNotScheduled,
				},
			},
		},
	}

	for i, tt := range tests {
		ar, err := NewReconciler(registry.NewFakeRegistry(), nil)
		if err != nil {
			t.Errorf("case %d: unexpected error from NewReconciler: %v", i, err)
			continue
		}

		taskchan := make(chan *task)
		tasks := []task{}
		go func() {
			ar.calculateTasksForJob(tt.dState, tt.cState, tt.jName, taskchan)
			close(taskchan)
		}()

		for t := range taskchan {
			tasks = append(tasks, *t)
		}

		if !reflect.DeepEqual(tt.tasks, tasks) {
			t.Errorf("case %d: calculated incorrect list of tasks\nexpected=%v\nreceived=%v\n", i, tt.tasks, tasks)
		}
	}
}
