package agent

import (
	"fmt"
	"reflect"
	"testing"
	"time"

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
			job:    &job.Job{Name: "easy-street.service", Unit: unit.UnitFile{}},
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

		chain *taskChain
	}{

		// nil agent state objects should result in no tasks
		{
			dState: nil,
			cState: nil,
			jName:  "foo.service",
			chain:  nil,
		},

		// nil job should result in no tasks
		{
			dState: NewAgentState(&machine.MachineState{ID: "XXX"}),
			cState: NewAgentState(&machine.MachineState{ID: "XXX"}),
			jName:  "foo.service",
			chain:  nil,
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
			chain: nil,
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
			chain: nil,
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
			chain: &taskChain{
				job: &job.Job{TargetState: jsLoaded},
				tasks: []task{
					task{
						typ:    taskTypeLoadJob,
						reason: taskReasonScheduledButUnloaded,
					},
				},
			},
		},

		// load + launch jobs that have a launched desired state
		{
			dState: &AgentState{
				MState: &machine.MachineState{ID: "XXX"},
				Jobs: map[string]*job.Job{
					"foo.service": &job.Job{TargetState: jsLaunched},
				},
			},
			cState: NewAgentState(&machine.MachineState{ID: "XXX"}),
			jName:  "foo.service",
			chain: &taskChain{
				job: &job.Job{TargetState: jsLaunched},
				tasks: []task{
					task{
						typ:    taskTypeLoadJob,
						reason: taskReasonScheduledButUnloaded,
					},
					task{
						typ:    taskTypeStartJob,
						reason: taskReasonLoadedDesiredStateLaunched,
					},
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
			chain: &taskChain{
				job: &job.Job{State: &jsLoaded},
				tasks: []task{
					task{
						typ:    taskTypeUnloadJob,
						reason: taskReasonLoadedButNotScheduled,
					},
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
			chain: &taskChain{
				job: &job.Job{State: &jsLaunched},
				tasks: []task{
					task{
						typ:    taskTypeUnloadJob,
						reason: taskReasonLoadedButNotScheduled,
					},
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
			chain: &taskChain{
				job: &job.Job{State: &jsLoaded},
				tasks: []task{
					task{
						typ:    taskTypeUnloadJob,
						reason: taskReasonLoadedButNotScheduled,
					},
				},
			},
		},
	}

	for i, tt := range tests {
		ar := NewReconciler(registry.NewFakeRegistry(), nil)
		chain := ar.calculateTaskChainForJob(tt.dState, tt.cState, tt.jName)
		if !reflect.DeepEqual(tt.chain, chain) {
			t.Errorf("case %d: calculated incorrect task chain\nexpected=%v\nreceived=%v\n", i, tt.chain, chain)
		}
	}
}

type FakeEventStream struct {
	ret chan registry.Event
}

func (f *FakeEventStream) Next(chan struct{}) chan registry.Event {
	return f.ret
}

func (f *FakeEventStream) trigger() {
	go func() {
		f.ret <- registry.JobTargetChangeEvent
	}()
}

// TestAgentReconcilerRun attempts to validate the behaviour of the central Run
// loop of the AgentReconciler, particularly its response to events
func TestAgentReconcilerRun(t *testing.T) {
	fes := &FakeEventStream{make(chan registry.Event)}
	called := make(chan struct{})
	rec := func(*AgentReconciler, *Agent) {
		go func() {
			called <- struct{}{}
		}()
	}
	ar := &AgentReconciler{
		reg:       nil,
		rStream:   fes,
		tManager:  nil,
		rint:      time.Hour, // Implausibly high reconcile interval we never expect to reach
		reconcile: rec,
	}
	// launch the AgentReconciler in the background
	arDone := make(chan bool)
	stop := make(chan bool)
	go func() {
		ar.Run(nil, stop)
		arDone <- true
	}()
	// no reconcile yet expected
	select {
	case <-called:
		t.Fatalf("reconcile() called unexpectedly!")
	default:
	}
	// now, send an event on the EventStream and ensure reconcile occurs
	fes.trigger()
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatalf("reconcile() not called after trigger!")
	}
	// assert reconcile was only called once
	select {
	case <-called:
		t.Fatalf("reconcile() called unexpectedly!")
	default:
	}
	// another event should work OK
	fes.trigger()
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatalf("reconcile() not called after trigger!")
	}
	// again, assert reconcile was only called once
	select {
	case <-called:
		t.Fatalf("reconcile() called unexpectedly!")
	default:
	}
	// stop the loop
	close(stop)
	// now, sending an event should do nothing
	fes.trigger()
	select {
	case <-called:
		t.Fatalf("reconcile() called unexpectedly!")
	default:
	}
	// and the AgentReconciler should have shut down
	select {
	case <-arDone:
	case <-time.After(time.Second):
		t.Fatalf("AgentReconciler.Run did not return after stop signal!")
	}
}
