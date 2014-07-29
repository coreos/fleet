package engine

import (
	"reflect"
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/pkg"
)

func TestSchedulerDecisions(t *testing.T) {
	tests := []struct {
		clust *clusterState
		job   *job.Job
		dec   *decision
	}{
		// job has no offer
		{
			clust: newClusterState([]job.Job{}, map[string]pkg.Set{}, []machine.MachineState{}),
			job:   &job.Job{Name: "foo.service"},
			dec:   nil,
		},

		// job has offer, but no bids
		{
			clust: newClusterState([]job.Job{}, map[string]pkg.Set{"foo.service": pkg.NewUnsafeSet()}, []machine.MachineState{}),
			job:   &job.Job{Name: "foo.service"},
			dec:   nil,
		},

		// job has offer and many bids, pick the first one
		{
			clust: newClusterState([]job.Job{}, map[string]pkg.Set{"foo.service": pkg.NewUnsafeSet("XXX", "YYY", "ZZZ")}, []machine.MachineState{}),
			job:   &job.Job{Name: "foo.service"},
			dec: &decision{
				machineID: "XXX",
			},
		},
	}

	for i, tt := range tests {
		sched := &dumbScheduler{}
		dec, err := sched.Decide(tt.clust, tt.job)

		if err != nil && tt.dec != nil {
			t.Errorf("case %d: unexpected error: %v", i, err)
			continue
		} else if err == nil && tt.dec == nil {
			t.Errorf("case %d: expected error", i)
			continue
		}

		if !reflect.DeepEqual(tt.dec, dec) {
			t.Errorf("case %d: expected decision %#v, got %#v", i, tt.dec, dec)
		}
	}
}
