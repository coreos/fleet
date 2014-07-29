package engine

import (
	"fmt"

	"github.com/coreos/fleet/job"
)

type decision struct {
	machineID string
}

type Scheduler interface {
	Decide(*clusterState, *job.Job) (*decision, error)
}

type dumbScheduler struct{}

func (ds *dumbScheduler) Decide(clust *clusterState, j *job.Job) (*decision, error) {
	bids, ok := clust.offers[j.Name]
	if !ok || bids.Length() == 0 {
		return nil, fmt.Errorf("no bids found, unable to decide")
	}

	dec := decision{
		machineID: bids.Values()[0],
	}

	return &dec, nil
}
