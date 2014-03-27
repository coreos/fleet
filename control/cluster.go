package control

import (
	"sort"

	log "github.com/coreos/fleet/third_party/github.com/golang/glog"
)

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

func (clus *cluster) JobScheduled(user string, jid string, host string, spec *JobSpec) {
	clus.mutex.Lock()
	defer clus.mutex.Unlock()

	clus.jobScheduled(host, spec)
}

func (clus *cluster) JobDowned(user string, jid string, host string, spec *JobSpec) {
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

func remainingFree(load, newLoad, total int) (float64, bool) {
	if load+newLoad > total {
		return 0.0, false
	}

	return float64(total-load-newLoad) / float64(total), true
}

func (clus *cluster) candidates(spec *JobSpec) ([]candHost, error) {
	candLoads := clus.loads

	if len(spec.RequiresHost) > 0 {
		host := spec.RequiresHost

		candLoads = make(map[string]MachineSpec, 1)
		candLoads[host] = clus.loads[host]

		if len(spec.DependsOn) > 0 {
			dependHosts, err := clus.hostsRunningAllJobs(spec.DependsOn)
			if err != nil {
				return nil, err
			}

			k := sort.SearchStrings(dependHosts, host)

			if k == len(dependHosts) || dependHosts[k] != host {
				return nil, ErrRequiredHostUnavailable
			}
		}
	} else if len(spec.DependsOn) > 0 {
		dependHosts, err := clus.hostsRunningAllJobs(spec.DependsOn)
		if err != nil {
			return nil, err
		}

		if len(dependHosts) == 0 {
			return nil, ErrDependOnHostUnavailable
		}

		candLoads = make(map[string]MachineSpec, len(dependHosts))
		for _, host := range dependHosts {
			candLoads[host] = clus.loads[host]
		}
	}

	clus.mutex.Lock()
	defer clus.mutex.Unlock()

	var lhs []candHost
	var lh candHost

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
	return lhs, nil
}
