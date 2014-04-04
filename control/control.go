package control

import "sync"

type candHost struct {
	mem   float64
	disk  float64
	cores float64
	score float64
	host  string
}

// We store an in-memory picture of load on each host but
// we don't store individual job stats because it's
// trickier to maintain and etcd already does it.
// The tradeoff here is that when we are asked to schedule
// jobs with dependsOn or conflictsWith clauses, we have to
// talk to etcd one more time.
// We believe jobs with those clauses are an exception, most
// jobs we schedule won't have them.

type cluster struct {
	mu       sync.Mutex
	loads    map[string]MachineSpec
	mdb      MachineDB
	etcd     Etcd
	strategy bestFitScoreMethod
}

func (clus *cluster) populate() error {
	clus.mu.Lock()
	defer clus.mu.Unlock()

	allJobs, err := clus.etcd.Jobs()
	if err != nil {
		return err
	}

	allHosts, err := clus.etcd.Hosts()
	if err != nil {
		return err
	}

	clus.loads = make(map[string]MachineSpec)

	var noLoad MachineSpec

	for _, h := range allHosts {
		clus.loads[h] = noLoad
	}

	for _, jwh := range allJobs {
		clus.jobScheduled(jwh.BootID, jwh.Spec)
	}
	return nil
}

// NewJobControl returns a newly created JobControl that will use
// the specified Etcd and the specified MachineDB.
func NewJobControl(etcd Etcd, mdb MachineDB) (JobControl, error) {
	clus := new(cluster)
	clus.etcd = etcd
	clus.mdb = mdb
	clus.strategy = sumScoreMethod

	err := clus.populate()
	if err != nil {
		return nil, err
	}
	return clus, nil
}

func (clus *cluster) ScheduleJob(spec *JobSpec) ([]string, error) {
	lhs, err := clus.candidates(spec)
	if err != nil {
		return nil, err
	}

	if len(lhs) == 0 {
		return nil, ErrClusterFull
	}

	sortBestFit(lhs, clus.strategy)

	bootIDs := make([]string, len(lhs))
	for i, v := range lhs {
		bootIDs[i] = v.host
	}
	return bootIDs, nil
}
