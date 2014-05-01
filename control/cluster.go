package control

import (
	"github.com/coreos/fleet/machine"
	log "github.com/coreos/fleet/third_party/github.com/golang/glog"
)

// these next methods keep the machine loads up to date with
// what happens in the cluster

func (clus *cluster) jobScheduled(bootID string, spec *JobSpec) {
	m := clus.loads[bootID]
	m.Cores += spec.CoresRequired
	m.DiskSpace += spec.DiskSpaceRequired
	m.Memory += spec.MemoryRequired
	clus.loads[bootID] = m
}

func (clus *cluster) jobDowned(bootID string, spec *JobSpec) {
	m := clus.loads[bootID]
	m.Cores -= spec.CoresRequired
	m.DiskSpace -= spec.DiskSpaceRequired
	m.Memory -= spec.MemoryRequired
	clus.loads[bootID] = m
}

func (clus *cluster) JobScheduled(jobName string, bootID string, spec *JobSpec) {
	clus.mu.Lock()
	defer clus.mu.Unlock()

	clus.jobScheduled(bootID, spec)
}

func (clus *cluster) JobDowned(jobName string, bootID string, spec *JobSpec) {
	clus.mu.Lock()
	defer clus.mu.Unlock()

	clus.jobDowned(bootID, spec)
}

func (clus *cluster) HostDown(bootID string) {
	clus.mu.Lock()
	defer clus.mu.Unlock()

	delete(clus.loads, bootID)
}

func (clus *cluster) HostUp(bootID string) {
	clus.mu.Lock()
	defer clus.mu.Unlock()

	var noLoad machine.MachineSpec
	clus.loads[bootID] = noLoad

	spec, err := clus.clusterCentral.Spec(bootID)
	if err != nil {
		log.Errorf("cannot determine machine spec for %s", bootID)
	} else {
		clus.specs[bootID] = *spec
	}
}

// Returns a list of host candidates where specified job could be
// scheduled. List has been filtered with respect to
// DependsOn, ConflictsWith and RequiresHost clauses in the job spec.
func (clus *cluster) candidates(spec *JobSpec) ([]candHost, error) {
	clus.mu.Lock()
	candLoads := make(map[string]machine.MachineSpec, len(clus.loads))
	for k, v := range clus.loads {
		candLoads[k] = v
	}
	clus.mu.Unlock()

	var lhs []candHost
	var lh candHost

	// first we look which machines can fit the job
	for host, load := range candLoads {
		mspec := clus.specs[host]

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

		v, ok = remainingFree(load.DiskSpace, spec.DiskSpaceRequired, mspec.DiskSpace)
		if !ok {
			continue
		}
		lh.disk = v

		lh.host = host
		lhs = append(lhs, lh)
	}

	if len(lhs) == 0 {
		return nil, ErrClusterFull
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
