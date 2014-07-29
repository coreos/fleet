package engine

import (
	"reflect"
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/pkg"
)

type fakeScheduler struct{}

func (fs *fakeScheduler) Decide(cs *clusterState, j *job.Job) (*decision, error) {
	dec := decision{
		machineID: "XXX",
	}
	return &dec, nil
}

func TestCalculateClusterTasks(t *testing.T) {
	jsInactive := job.JobStateInactive
	jsLaunched := job.JobStateLaunched

	tests := []struct {
		clust *clusterState
		tasks []*task
	}{
		{
			clust: newClusterState([]job.Job{}, map[string]pkg.Set{}, []machine.MachineState{}),
			tasks: []*task{},
		},

		// do nothing if Job is shcheduled and target machine exists
		{
			clust: newClusterState(
				[]job.Job{
					job.Job{
						Name:            "foo.service",
						TargetState:     job.JobStateLaunched,
						State:           &jsLaunched,
						TargetMachineID: "XXX",
					},
				},
				map[string]pkg.Set{},
				[]machine.MachineState{
					machine.MachineState{ID: "XXX"},
				},
			),
			tasks: []*task{},
		},

		// clean up job offers if Job is healthy
		{
			clust: newClusterState(
				[]job.Job{
					job.Job{
						Name:            "foo.service",
						TargetState:     job.JobStateLaunched,
						State:           &jsLaunched,
						TargetMachineID: "XXX",
					},
				},
				map[string]pkg.Set{
					"foo.service": pkg.NewUnsafeSet(),
				},
				[]machine.MachineState{
					machine.MachineState{ID: "XXX"},
				},
			),
			tasks: []*task{
				&task{
					Type:   taskTypeResolveOffer,
					Reason: "already scheduled",
					Job: &job.Job{
						Name: "foo.service",
					},
				},
			},
		},

		// reschedule if Job's target machine is gone
		{
			clust: newClusterState(
				[]job.Job{
					job.Job{
						Name:            "foo.service",
						TargetState:     job.JobStateLaunched,
						State:           &jsLaunched,
						TargetMachineID: "XXX",
					},
				},
				map[string]pkg.Set{},
				[]machine.MachineState{
					machine.MachineState{ID: "YYY"},
				},
			),
			tasks: []*task{
				&task{
					Type:   taskTypeUnscheduleJob,
					Reason: "target Machine(XXX) went away",
					Job: &job.Job{
						Name:            "foo.service",
						TargetState:     job.JobStateLaunched,
						State:           &jsLaunched,
						TargetMachineID: "XXX",
					},
				},
				&task{
					Type:   taskTypeOfferJob,
					Reason: "target state launched and Job not scheduled",
					Job: &job.Job{
						Name:            "foo.service",
						TargetState:     job.JobStateLaunched,
						State:           &jsLaunched,
						TargetMachineID: "XXX",
					},
				},
			},
		},

		// unschedule if Job's target state inactive and is scheduled
		{
			clust: newClusterState(
				[]job.Job{
					job.Job{
						Name:            "foo.service",
						TargetState:     job.JobStateInactive,
						State:           &jsLaunched,
						TargetMachineID: "XXX",
					},
				},
				map[string]pkg.Set{},
				[]machine.MachineState{
					machine.MachineState{ID: "XXX"},
				},
			),
			tasks: []*task{
				&task{
					Type:   taskTypeUnscheduleJob,
					Reason: "target state inactive",
					Job: &job.Job{
						Name:            "foo.service",
						TargetState:     job.JobStateInactive,
						State:           &jsLaunched,
						TargetMachineID: "XXX",
					},
				},
			},
		},

		// remove offer if target state inactive
		{
			clust: newClusterState(
				[]job.Job{
					job.Job{
						Name:        "foo.service",
						TargetState: job.JobStateInactive,
						State:       &jsLaunched,
					},
				},
				map[string]pkg.Set{
					"foo.service": pkg.NewUnsafeSet(),
				},
				[]machine.MachineState{
					machine.MachineState{ID: "XXX"},
				},
			),
			tasks: []*task{
				&task{
					Type:   taskTypeResolveOffer,
					Reason: "target state inactive",
					Job: &job.Job{
						Name: "foo.service",
					},
				},
			},
		},

		// remove offer if corresponding job does not exist
		{
			clust: newClusterState(
				[]job.Job{},
				map[string]pkg.Set{
					"foo.service": pkg.NewUnsafeSet(),
				},
				[]machine.MachineState{
					machine.MachineState{ID: "XXX"},
				},
			),
			tasks: []*task{
				&task{
					Type:   taskTypeResolveOffer,
					Reason: "job does not exist",
					Job: &job.Job{
						Name: "foo.service",
					},
				},
			},
		},

		// offer a Job where TargetState != State and no offer exists
		{
			clust: newClusterState(
				[]job.Job{
					job.Job{
						Name:        "foo.service",
						TargetState: job.JobStateLaunched,
						State:       &jsInactive,
					},
				},
				map[string]pkg.Set{},
				[]machine.MachineState{
					machine.MachineState{ID: "XXX"},
				},
			),
			tasks: []*task{
				&task{
					Type:   taskTypeOfferJob,
					Reason: "target state launched and Job not scheduled",
					Job: &job.Job{
						Name:        "foo.service",
						TargetState: job.JobStateLaunched,
						State:       &jsInactive,
					},
				},
			},
		},

		// attempt to schedule a Job if an offer exists
		{
			clust: newClusterState(
				[]job.Job{
					job.Job{
						Name:            "foo.service",
						TargetState:     job.JobStateLaunched,
						State:           &jsInactive,
						TargetMachineID: "",
					},
				},
				map[string]pkg.Set{
					"foo.service": pkg.NewUnsafeSet("XXX"),
				},
				[]machine.MachineState{
					machine.MachineState{ID: "XXX"},
				},
			),
			tasks: []*task{
				&task{
					Type:   taskTypeAttemptScheduleJob,
					Reason: "target state launched and Job not scheduled",
					Job: &job.Job{
						Name:            "foo.service",
						TargetState:     job.JobStateLaunched,
						State:           &jsInactive,
						TargetMachineID: "XXX",
					},
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
