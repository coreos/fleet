package control

import log "github.com/coreos/fleet/third_party/github.com/golang/glog"

func (clus *cluster) jobScheduled(host HostID, spec *JobSpec) {
	m := clus.loads[host]
	m.Cores += spec.CoresRequired
	m.LocalDiskSpace += spec.LocalDiskSpaceRequired
	m.Memory += spec.MemoryRequired
	clus.loads[host] = m
}

func (clus *cluster) jobDowned(host HostID, spec *JobSpec) {
	m := clus.loads[host]
	m.Cores -= spec.CoresRequired
	m.LocalDiskSpace -= spec.LocalDiskSpaceRequired
	m.Memory -= spec.MemoryRequired
	clus.loads[host] = m
}

func (clus *cluster) JobScheduled(user UserID, jid JobID, host HostID, spec *JobSpec) {
	clus.mutex.Lock()
	defer clus.mutex.Unlock()

	clus.jobScheduled(host, spec)
}

func (clus *cluster) JobDowned(user UserID, jid JobID, host HostID, spec *JobSpec) {
	clus.mutex.Lock()
	defer clus.mutex.Unlock()

	clus.jobDowned(host, spec)
}

func (clus *cluster) HostDown(host HostID) {
	clus.mutex.Lock()
	defer clus.mutex.Unlock()

	delete(clus.loads, host)
}

func remainingFree(load, newLoad, total int) (float64, bool) {
	if load+newLoad > total {
		return 0.0, false
	}

	return float64(total-load-newLoad) / float64(total), true
}

func (clus *cluster) candidates(spec *JobSpec) ([]candHost, error) {
	clus.mutex.Lock()
	defer clus.mutex.Unlock()

	var lhs []candHost
	var lh candHost

	for host, _ := range clus.loads {
		load := clus.loads[host]

		mspec, err := clus.mdb.Spec(host)
		if err != nil {
			log.Errorf("unable to get machine spec for %v: %v, skipping scheduling for it", host, err)
			continue
		}

		v, ok := remainingFree(load.Cores, spec.CoresRequired, mspec.Cores)
		if !ok {
			continue
		}
		lh.cores = v

		v, ok = remainingFree(load.Memory, spec.MemoryRequired, mspec.Memory)
		if !ok {
			continue
		}
		lh.mem = v

		v, ok = remainingFree(load.LocalDiskSpace, spec.LocalDiskSpaceRequired, mspec.LocalDiskSpace)
		if !ok {
			continue
		}
		lh.disk = v

		lh.host = host
		lhs = append(lhs, lh)
	}
	return lhs, nil
}
