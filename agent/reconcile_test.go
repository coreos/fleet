package agent

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/pkg"
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
		dState *agentState
		mState *machine.MachineState
		job    *job.Job
		want   bool
	}{
		// nothing to worry about
		{
			dState: newAgentState(),
			mState: &machine.MachineState{ID: "123"},
			job:    &job.Job{Name: "easy-street.service", Unit: unit.Unit{}},
			want:   true,
		},

		// match X-ConditionMachineID
		{
			dState: newAgentState(),
			mState: &machine.MachineState{ID: "XYZ"},
			job:    newTestJobWithXFleetValues(t, "X-ConditionMachineID=XYZ"),
			want:   true,
		},

		// mismatch X-ConditionMachineID
		{
			dState: newAgentState(),
			mState: &machine.MachineState{ID: "123"},
			job:    newTestJobWithXFleetValues(t, "X-ConditionMachineID=XYZ"),
			want:   false,
		},

		// match X-ConditionMachineMetadata
		{
			dState: newAgentState(),
			mState: &machine.MachineState{ID: "123", Metadata: map[string]string{"region": "us-west"}},
			job:    newTestJobWithXFleetValues(t, "X-ConditionMachineMetadata=region=us-west"),
			want:   true,
		},

		// Machine metadata ignored when no X-ConditionMachineMetadata in Job
		{
			dState: newAgentState(),
			mState: &machine.MachineState{ID: "123", Metadata: map[string]string{"region": "us-west"}},
			job:    &job.Job{Name: "easy-street.service", Unit: unit.Unit{}},
			want:   true,
		},

		// mismatch X-ConditionMachineMetadata
		{
			dState: newAgentState(),
			mState: &machine.MachineState{ID: "123", Metadata: map[string]string{"region": "us-west"}},
			job:    newTestJobWithXFleetValues(t, "X-ConditionMachineMetadata=region=us-east"),
			want:   false,
		},

		// peer scheduled locally
		{
			dState: &agentState{
				jobs: map[string]*job.Job{
					"pong.service": &job.Job{Name: "pong.service"},
				},
			},
			mState: &machine.MachineState{ID: "123"},
			job:    newTestJobWithXFleetValues(t, "X-ConditionMachineOf=pong.service"),
			want:   true,
		},

		{
			dState: &agentState{
				jobs: map[string]*job.Job{
					"ping.pong.service": &job.Job{Name: "ping.pong.service"},
				},
			},
			mState: &machine.MachineState{ID: "123"},
			job:    newTestJobWithXFleetValues(t, "X-ConditionMachineOf=ping.*.service"),
			want:   true,
		},

		// multiple peers scheduled locally
		{
			dState: &agentState{
				jobs: map[string]*job.Job{
					"ping.service": &job.Job{Name: "ping.service"},
					"pong.service": &job.Job{Name: "pong.service"},
				},
			},
			mState: &machine.MachineState{ID: "123"},
			job:    newTestJobWithXFleetValues(t, "X-ConditionMachineOf=pong.service\nX-ConditionMachineOf=ping.service"),
			want:   true,
		},

		// peer not scheduled locally
		{
			dState: newAgentState(),
			mState: &machine.MachineState{ID: "123"},
			job:    newTestJobWithXFleetValues(t, "X-ConditionMachineOf=ping.service"),
			want:   false,
		},

		// one of multiple peers not scheduled locally
		{
			dState: &agentState{
				jobs: map[string]*job.Job{
					"ping.service": &job.Job{Name: "ping.service"},
				},
			},
			mState: &machine.MachineState{ID: "123"},
			job:    newTestJobWithXFleetValues(t, "X-ConditionMachineOf=pong.service\nX-ConditionMachineOf=ping.service"),
			want:   false,
		},

		// no conflicts found
		{
			dState: &agentState{
				jobs: map[string]*job.Job{
					"ping.service": &job.Job{Name: "ping.service"},
				},
			},
			mState: &machine.MachineState{ID: "123"},
			job:    newTestJobWithXFleetValues(t, "X-Conflicts=pong.service"),
			want:   true,
		},

		// conflicts found
		{
			dState: &agentState{
				jobs: map[string]*job.Job{
					"ping.service": &job.Job{Name: "ping.service"},
				},
			},
			mState: &machine.MachineState{ID: "123"},
			job:    newTestJobWithXFleetValues(t, "X-Conflicts=ping.service"),
			want:   false,
		},
	}

	for i, tt := range tests {
		ar := NewReconciler(registry.NewFakeRegistry(), nil)
		got, _ := ar.ableToRun(tt.dState, tt.mState, tt.job)
		if got != tt.want {
			t.Errorf("case %d: expected %t, got %t", i, tt.want, got)
		}
	}
}

func TestCalculateTasksForJob(t *testing.T) {
	tests := []struct {
		mState *machine.MachineState
		dState *agentState
		cState *agentState
		jName  string

		tasks []task
	}{

		// nil agent state objects should result in no tasks
		{
			mState: &machine.MachineState{ID: "XXX"},
			dState: nil,
			cState: nil,
			jName:  "foo.service",
			tasks:  []task{},
		},

		// nil job should result in no tasks
		{
			mState: &machine.MachineState{ID: "XXX"},
			dState: newAgentState(),
			cState: newAgentState(),
			jName:  "foo.service",
			tasks:  []task{},
		},

		// no work needs to be done when target state == desired state
		{
			mState: &machine.MachineState{ID: "XXX"},
			dState: &agentState{
				jobs: map[string]*job.Job{
					"foo.service": &job.Job{TargetState: jsLoaded},
				},
			},
			cState: &agentState{
				jobs: map[string]*job.Job{
					"foo.service": &job.Job{State: &jsLoaded},
				},
			},
			jName: "foo.service",
			tasks: []task{},
		},

		// no work needs to be done when target state == desired state
		{
			mState: &machine.MachineState{ID: "XXX"},
			dState: &agentState{
				jobs: map[string]*job.Job{
					"foo.service": &job.Job{TargetState: jsLaunched},
				},
			},
			cState: &agentState{
				jobs: map[string]*job.Job{
					"foo.service": &job.Job{State: &jsLaunched},
				},
			},
			jName: "foo.service",
			tasks: []task{},
		},

		// load jobs that have a loaded desired state
		{
			mState: &machine.MachineState{ID: "XXX"},
			dState: &agentState{
				jobs: map[string]*job.Job{
					"foo.service": &job.Job{TargetState: jsLoaded},
				},
			},
			cState: newAgentState(),
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
			mState: &machine.MachineState{ID: "XXX"},
			dState: &agentState{
				jobs: map[string]*job.Job{
					"foo.service": &job.Job{TargetState: jsLaunched},
				},
			},
			cState: newAgentState(),
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
			mState: &machine.MachineState{ID: "XXX"},
			dState: newAgentState(),
			cState: &agentState{
				jobs: map[string]*job.Job{
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
			mState: &machine.MachineState{ID: "XXX"},
			dState: newAgentState(),
			cState: &agentState{
				jobs: map[string]*job.Job{
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
			mState: &machine.MachineState{ID: "XXX"},
			dState: &agentState{
				jobs: map[string]*job.Job{
					"foo.service": &job.Job{
						TargetState: jsInactive,
					},
				},
			},
			cState: &agentState{
				jobs: map[string]*job.Job{
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

		// unschedule jobs that can not run locally
		{
			mState: &machine.MachineState{ID: "XXX"},
			dState: &agentState{
				jobs: map[string]*job.Job{
					"foo.service": &job.Job{
						TargetState: jsLaunched,
						Unit:        fleetUnit(t, "X-ConditionMachineID=YYY"),
					},
				},
			},
			cState: newAgentState(),
			jName:  "foo.service",
			tasks: []task{
				task{
					Type: taskTypeUnscheduleJob,
					Job: &job.Job{
						TargetState: jsLaunched,
						Unit:        fleetUnit(t, "X-ConditionMachineID=YYY"),
					},
					Reason: taskReasonScheduledButNotRunnable,
				},
				task{
					Type: taskTypeUnloadJob,
					Job: &job.Job{
						TargetState: jsLaunched,
						Unit:        fleetUnit(t, "X-ConditionMachineID=YYY"),
					},
					Reason: taskReasonScheduledButNotRunnable,
				},
			},
		},
	}

	for i, tt := range tests {
		ar := NewReconciler(registry.NewFakeRegistry(), nil)
		taskchan := make(chan *task)
		tasks := []task{}
		go func() {
			ar.calculateTasksForJob(tt.mState, tt.dState, tt.cState, tt.jName, taskchan)
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

func TestCalculateTasksForOffer(t *testing.T) {
	tests := []struct {
		mState *machine.MachineState
		dState *agentState
		job    *job.Job
		bids   pkg.Set

		tasks []task
	}{
		// no bid submitted yet and able to run
		{
			mState: &machine.MachineState{ID: "XXX"},
			dState: newAgentState(),
			job: &job.Job{
				Name:        "foo.service",
				TargetState: jsLaunched,
				Unit:        fleetUnit(t),
			},
			bids: pkg.NewUnsafeSet(),
			tasks: []task{
				task{
					Type: taskTypeSubmitBid,
					Job: &job.Job{
						Name:        "foo.service",
						TargetState: jsLaunched,
						Unit:        fleetUnit(t),
					},
					Reason: taskReasonAbleToResolveOffer,
				},
			},
		},

		// no bid submitted but unable to run
		{
			mState: &machine.MachineState{ID: "XXX"},
			dState: newAgentState(),
			job: &job.Job{
				Name:        "foo.service",
				TargetState: jsLaunched,
				Unit:        fleetUnit(t, "X-ConditionMachineID=YYY"),
			},
			bids:  pkg.NewUnsafeSet(),
			tasks: []task{},
		},

		// bid already submitted
		{
			mState: &machine.MachineState{ID: "XXX"},
			dState: newAgentState(),
			job: &job.Job{
				TargetState: jsLaunched,
				Unit:        fleetUnit(t),
			},
			bids:  pkg.NewUnsafeSet("XXX"),
			tasks: []task{},
		},
	}

	for i, tt := range tests {
		ar := NewReconciler(registry.NewFakeRegistry(), nil)
		taskchan := make(chan *task)
		tasks := []task{}
		go func() {
			ar.calculateTasksForOffer(tt.dState, tt.mState, tt.job, tt.bids, taskchan)
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
