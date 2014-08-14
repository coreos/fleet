package engine

import (
	"reflect"
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
)

func TestCalculateClusterTasks(t *testing.T) {
	jsInactive := job.JobStateInactive
	jsLaunched := job.JobStateLaunched

	tests := []struct {
		clust *clusterState
		tasks []*task
	}{
		// no work to do
		{
			clust: newClusterState([]job.Unit{}, []job.ScheduledUnit{}, []machine.MachineState{}),
			tasks: []*task{},
		},

		// do nothing if Job is shcheduled and target machine exists
		{
			clust: newClusterState(
				[]job.Unit{
					job.Unit{
						Name:        "foo.service",
						TargetState: job.JobStateLaunched,
					},
				},
				[]job.ScheduledUnit{
					job.ScheduledUnit{
						Name:            "foo.service",
						State:           &jsLaunched,
						TargetMachineID: "XXX",
					},
				},
				[]machine.MachineState{
					machine.MachineState{ID: "XXX"},
				},
			),
			tasks: []*task{},
		},

		// reschedule if Job's target machine is gone
		{
			clust: newClusterState(
				[]job.Unit{
					job.Unit{
						Name:        "foo.service",
						TargetState: job.JobStateLaunched,
					},
				},
				[]job.ScheduledUnit{
					job.ScheduledUnit{
						Name:            "foo.service",
						State:           &jsLaunched,
						TargetMachineID: "ZZZ",
					},
				},
				[]machine.MachineState{
					machine.MachineState{ID: "XXX"},
				},
			),
			tasks: []*task{
				&task{
					Type:      taskTypeUnscheduleJob,
					Reason:    "target Machine(ZZZ) went away",
					JobName:   "foo.service",
					MachineID: "ZZZ",
				},
				&task{
					Type:      taskTypeAttemptScheduleUnit,
					Reason:    "target state launched and unit not scheduled",
					JobName:   "foo.service",
					MachineID: "XXX",
				},
			},
		},

		// unschedule if Job's target state inactive and is scheduled
		{
			clust: newClusterState(
				[]job.Unit{
					job.Unit{
						Name:        "foo.service",
						TargetState: job.JobStateInactive,
					},
				},
				[]job.ScheduledUnit{
					job.ScheduledUnit{
						Name:            "foo.service",
						State:           &jsLaunched,
						TargetMachineID: "XXX",
					},
				},
				[]machine.MachineState{
					machine.MachineState{ID: "XXX"},
				},
			),
			tasks: []*task{
				&task{
					Type:      taskTypeUnscheduleJob,
					Reason:    "target state inactive",
					JobName:   "foo.service",
					MachineID: "XXX",
				},
			},
		},

		// attempt to schedule a Job if a machine exists
		{
			clust: newClusterState(
				[]job.Unit{
					job.Unit{
						Name:        "foo.service",
						TargetState: job.JobStateLaunched,
					},
				},
				[]job.ScheduledUnit{
					job.ScheduledUnit{
						Name:            "foo.service",
						State:           &jsInactive,
						TargetMachineID: "",
					},
				},
				[]machine.MachineState{
					machine.MachineState{ID: "XXX"},
				},
			),
			tasks: []*task{
				&task{
					Type:      taskTypeAttemptScheduleUnit,
					Reason:    "target state launched and unit not scheduled",
					JobName:   "foo.service",
					MachineID: "XXX",
				},
			},
		},
	}

	for i, tt := range tests {
		r := NewReconciler()
		tasks := make([]*task, 0)
		for tsk := range r.calculateClusterTasks(tt.clust, make(chan struct{})) {
			tasks = append(tasks, tsk)
		}

		if !reflect.DeepEqual(tt.tasks, tasks) {
			t.Errorf("case %d: task mismatch\nexpected %v\n got %v", i, tt.tasks, tasks)
		}
	}
}
