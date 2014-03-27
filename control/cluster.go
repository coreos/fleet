package control

import log "github.com/coreos/fleet/third_party/github.com/golang/glog"

// these next methods keep the machine loads up to date with
// what happens in the cluster

func (clus *cluster) jobScheduled(host string, spec *JobSpec) {
	m := clus.loads[host]
	m.Cores += spec.CoresRequired
	m.LocalDiskSpace += spec.LocalDiskSpaceRequired
	m.Memory += spec.MemoryRequired
	clus.loads[host] = m
}

func (clus *cluster) jobDowned(host string, spec *JobSpec) {
	m := clus.loads[host]
	m.Cores -= spec.CoresRequired
	m.LocalDiskSpace -= spec.LocalDiskSpaceRequired
	m.Memory -= spec.MemoryRequired
	clus.loads[host] = m
}

func (clus *cluster) JobScheduled(jid string, host string, spec *JobSpec) {
	clus.mutex.Lock()
	defer clus.mutex.Unlock()

	clus.jobScheduled(host, spec)
}

func (clus *cluster) JobDowned(jid string, host string, spec *JobSpec) {
	clus.mutex.Lock()
	defer clus.mutex.Unlock()

	clus.jobDowned(host, spec)
}

func (clus *cluster) HostDown(host string) {
	clus.mutex.Lock()
	defer clus.mutex.Unlock()

	delete(clus.loads, host)
}

func (clus *cluster) HostUp(host string) {
	clus.mutex.Lock()
	defer clus.mutex.Unlock()

	var noLoad MachineSpec

	clus.loads[host] = noLoad
}

// Returns a list of host candidates where specified job could be
// scheduled. List has  been filtered with respect to
// DependsOn, ConflictsWith and RequiresHost clauses in the job spec.
func (clus *cluster) candidates(spec *JobSpec) ([]candHost, error) {
	clus.mutex.Lock()
	candLoads := make(map[string]MachineSpec, len(clus.loads))
	for k, v := range clus.loads {
		candLoads[k] = v
	}
	clus.mutex.Unlock()

	var lhs []candHost
	var lh candHost

	// first we look which machines can fit the job
	for host, load := range candLoads {
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

	// we also need to filter on DependsOn, ConflictsWith and RequiresHost clauses
	return clus.filterCandidates(lhs, spec)
}

func remainingFree(load, newLoad, total int) (float64, bool) {
	if load+newLoad > total {
		return 0.0, false
	}

	return float64(total-load-newLoad) / float64(total), true
}
