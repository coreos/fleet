package engine

import (
	"fmt"
	"sort"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/pkg"
)

type decision struct {
	machineID string
}

type Scheduler interface {
	Decide(*clusterState, *job.Job) (*decision, error)
}

type dumbScheduler struct{}

func (ds *dumbScheduler) Decide(clust *clusterState, j *job.Job) (*decision, error) {
	if len(clust.agents) == 0 {
		return nil, fmt.Errorf("zero agents available")
	}

	able := pkg.NewUnsafeSet()
	for machID, as := range clust.agents {
		if ok, _ := as.AbleToRun(j); !ok {
			continue
		}

		able.Add(machID)
	}

	if able.Length() == 0 {
		return nil, fmt.Errorf("no agents able to run job")
	}

	sorted := able.Values()
	sort.Strings(sorted)

	dec := decision{
		machineID: sorted[0],
	}

	return &dec, nil
}
