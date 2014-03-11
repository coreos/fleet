package engine

import (
	"sort"
	"sync"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/unit"
)

const (
	partitionSize = 5
)

type countbyname map[string]int

func (cbn countbyname) inc(key string) {
	v, ok := cbn[key]
	if !ok {
		cbn[key] = 0
	}
	cbn[key] = v + 1
}

func (cbn countbyname) dec(key string) {
	v, ok := cbn[key]
	if !ok {
		return
	}
	cbn[key] = v - 1
}

type namedCount struct {
	name  string
	count int
}

type namedCounts []*namedCount

func (a namedCounts) Len() int           { return len(a) }
func (a namedCounts) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a namedCounts) Less(i, j int) bool { return a[i].count < a[j].count }

type cluster struct {
	uptodate bool
	mutex    *sync.Mutex

	// some statistics about what goes on in the cluster
	// this will get more sophisticated as needed

	// where are jobs running
	jobsToMachines map[string]string
	// how many jobs on a machine
	machineJobCount countbyname
}

func newCluster() *cluster {
	return &cluster{false, new(sync.Mutex), make(map[string]string), make(map[string]int)}
}

// jobScheduled handles the job scheduled event
func (clust *cluster) jobScheduled(jobName string, mst *machine.MachineState) {
	clust.mutex.Lock()
	defer clust.mutex.Unlock()

	if !clust.uptodate {
		return
	}

	clust.populateJob(jobName, mst.BootId)
}

// jobStopped handles the job stopped event
func (clust *cluster) jobStopped(jobName string) {
	clust.mutex.Lock()
	defer clust.mutex.Unlock()

	if !clust.uptodate {
		return
	}

	clust.deleteJob(jobName)
}

// isUptodate returns whether the cluster has been told what happened before it got created
func (clust *cluster) isUptodate() bool {
	clust.mutex.Lock()
	defer clust.mutex.Unlock()
	return clust.uptodate
}

// refreshFrom refreshes from a specified cluster
func (clust *cluster) refreshFrom(cu *cluster, force bool) {
	clust.mutex.Lock()
	defer clust.mutex.Unlock()

	if !force && clust.uptodate {
		return
	}

	clust.jobsToMachines = cu.jobsToMachines
	clust.machineJobCount = cu.machineJobCount
	clust.uptodate = true
}

func (clust *cluster) populateJob(jobName string, machineBootID string) {
	clust.jobsToMachines[jobName] = machineBootID
	clust.machineJobCount.inc(machineBootID)
}

func (clust *cluster) deleteJob(jobName string) {
	machineBootID, ok := clust.jobsToMachines[jobName]

	// TODO(uwedeportivo): this might be a signal that a refresh is needed
	if !ok {
		return
	}

	clust.machineJobCount.dec(machineBootID)
}

func (clust *cluster) kLeastLoaded(k int) []string {
	clust.mutex.Lock()
	defer clust.mutex.Unlock()

	mas := make(namedCounts, len(clust.machineJobCount))
	cursor := 0
	for k, v := range clust.machineJobCount {
		mas[cursor] = &namedCount{k, v}
		cursor++
	}
	sort.Sort(mas)

	l := k
	if l > len(mas) {
		l = len(mas)
	}

	mbis := make([]string, l)
	for i, nc := range mas {
		mbis[i] = nc.name
	}
	return mbis
}

func (eg *Engine) refreshCluster(force bool) {
	if !force && eg.clust.isUptodate() {
		return
	}

	cu := newCluster()

	jobs := eg.registry.GetAllJobs()
	for _, j := range jobs {
		mst := eg.registry.GetJobTarget(j.Name)
		cu.populateJob(j.Name, mst.BootId)
	}

	eg.clust.refreshFrom(cu, force)
}

// requiresMachine returns whether specified job requires a specific machine.
func (eg *Engine) requiresMachine(j *job.Job) ([]string, bool) {
	requirements := j.Requirements()

	bootID, ok := requirements[unit.FleetXConditionMachineBootID]
	return bootID, ok && len(bootID) > 0
}

// partitionCluster returns a slice of bootids from a subset of active machines
// that should be considered for scheduling the specified job.
// The returned slice is sorted by ascending lexicographical string value of machine boot id.
func (eg *Engine) partitionCluster(j *job.Job) ([]string, error) {
	if bootID, ok := eg.requiresMachine(j); ok {
		return bootID, nil
	}

	// TODO(uwedeportivo): for now punt on jobs with requirements and offer to all machines
	// because agents are decoding the requirements
	if len(j.Requirements()) > 0 {
		machines := eg.registry.GetActiveMachines()

		machineBootIds := make([]string, len(machines))
		for i, mach := range machines {
			machineBootIds[i] = mach.BootId
		}
		sort.Strings(machineBootIds)
		return machineBootIds, nil
	}

	// this is usually a cheap no-op
	eg.refreshCluster(false)

	// as an initial heuristic, choose the k least loaded, with k = partitionSize
	machineBootIds := eg.clust.kLeastLoaded(partitionSize)

	sort.Strings(machineBootIds)
	return machineBootIds, nil
}
