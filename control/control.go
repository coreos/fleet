package control

import (
	"sync"

	uuid "github.com/coreos/fleet/third_party/code.google.com/p/go-uuid/uuid"
	log "github.com/coreos/fleet/third_party/github.com/golang/glog"
)

type candHost struct {
	mem   float64
	disk  float64
	cores float64
	host  HostID
}

type cluster struct {
	mutex    sync.Mutex
	loads    map[HostID]MachineSpec
	mdb      MachineDB
	etcd     Etcd
	strategy bestFitScoreMethod
}

func (clus *cluster) populate() error {
	clus.mutex.Lock()
	defer clus.mutex.Unlock()

	allJobs, err := clus.etcd.AllJobs()
	if err != nil {
		return err
	}

	allHosts, err := clus.etcd.AllHosts()
	if err != nil {
		return err
	}

	clus.loads = make(map[HostID]MachineSpec)

	var noLoad MachineSpec

	for _, h := range allHosts {
		clus.loads[h] = noLoad
	}

	for _, jwh := range allJobs {
		clus.jobScheduled(jwh.Host, jwh.Spec)
	}
	return nil
}

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

func (clus *cluster) ScheduleJob(user UserID, spec *JobSpec) (JobID, error) {
	lhs, err := clus.candidates(spec)
	if err != nil {
		return "", err
	}

	if len(lhs) == 0 {
		return "", ErrClusterFull
	}

	lhs, err = clus.filterCandidates(lhs, user, spec)
	if err != nil {
		return "", err
	}

	lhs = strategies[clus.strategy](lhs)

	jid := JobID(uuid.New())

	for _, h := range lhs {
		ag, err := clus.etcd.HostAgent(h.host)
		if err != nil {
			log.Errorf("failed to get host agent %v: %v, skipping to next host", h.host, err)
			continue
		}

		err = ag.RunJob(user, jid, spec)
		if err != nil {
			log.Errorf("failed to run job on host %v: %v, skipping to next host", h.host, err)
			continue
		}
		return jid, nil
	}

	return "", ErrAllAgentsFailedToRun
}
