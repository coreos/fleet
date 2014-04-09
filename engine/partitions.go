package engine

import (
	"sort"
	"sync"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
)

const (
	partitionSize = 5
)

type namedCount struct {
	name  string
	count int
}

type namedCounts []*namedCount

func (a namedCounts) Len() int           { return len(a) }
func (a namedCounts) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a namedCounts) Less(i, j int) bool { return a[i].count < a[j].count }

type cluster struct {
	upToDate bool
	mutex    sync.Mutex

	// some statistics about what goes on in the cluster
	// this will get more sophisticated as needed

	// where are jobs running
	jobsToMachines map[string]string
	// how many jobs on a machine
	// each active machine has at least one job,
	// otherwise it won't show up here
	machineJobCount map[string]int
}

func newCluster() *cluster {
	return &cluster{
		jobsToMachines:  make(map[string]string),
		machineJobCount: make(map[string]int),
	}
}

func (clust *cluster) machineCreated(bootID string) {
	clust.mutex.Lock()
	defer clust.mutex.Unlock()

	if !clust.upToDate {
		return
	}

	clust.populateMachine(bootID)
}

func (clust *cluster) machineRemoved(bootID string) {
	clust.mutex.Lock()
	defer clust.mutex.Unlock()

	if !clust.upToDate {
		return
	}

	clust.deleteMachine(bootID)
}

// jobScheduled handles the job scheduled event
func (clust *cluster) jobScheduled(jobName string, mst *machine.MachineState) {
	clust.mutex.Lock()
	defer clust.mutex.Unlock()

	if !clust.upToDate {
		return
	}

	clust.populateJob(jobName, mst.BootID)
}

// jobStopped handles the job stopped event
func (clust *cluster) jobStopped(jobName string) {
	clust.mutex.Lock()
	defer clust.mutex.Unlock()

	if !clust.upToDate {
		return
	}

	clust.deleteJob(jobName)
}

// isUpToDate returns whether the cluster has been told what happened before it got created
func (clust *cluster) isUpToDate() bool {
	clust.mutex.Lock()
	defer clust.mutex.Unlock()
	return clust.upToDate
}

// refreshFrom refreshes from a specified cluster
func (clust *cluster) refreshFrom(cu *cluster, force bool) {
	clust.mutex.Lock()
	defer clust.mutex.Unlock()

	if !force && clust.upToDate {
		return
	}

	clust.jobsToMachines = cu.jobsToMachines
	clust.machineJobCount = cu.machineJobCount
	clust.upToDate = true
}

func (clust *cluster) populateJob(jobName string, machineBootID string) {
	clust.jobsToMachines[jobName] = machineBootID
	clust.machineJobCount[machineBootID] = clust.machineJobCount[machineBootID] + 1
}

func (clust *cluster) deleteJob(jobName string) {
	machineBootID, ok := clust.jobsToMachines[jobName]

	// TODO(uwedeportivo): this might be a signal that a refresh is needed
	if !ok {
		return
	}

	clust.machineJobCount[machineBootID] = clust.machineJobCount[machineBootID] - 1
	delete(clust.jobsToMachines, jobName)
}

func (clust *cluster) populateMachine(bootID string) {
	clust.machineJobCount[bootID] = 1
}

func (clust *cluster) deleteMachine(bootID string) {
	delete(clust.machineJobCount, bootID)
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
	for i := 0; i < l; i++ {
		mbis[i] = mas[i].name
	}
	return mbis
}

func (eg *Engine) refreshCluster(force bool) {
	if !force && eg.clust.isUpToDate() {
		return
	}

	cu := newCluster()

	ms := eg.registry.GetActiveMachines()
	for _, m := range ms {
		cu.populateMachine(m.BootID)
	}

	jobs := eg.registry.GetAllJobs()
	for _, j := range jobs {
		tgt := eg.registry.GetJobTarget(j.Name)
		if tgt != "" {
			cu.populateJob(j.Name, tgt)
		}
	}

	eg.clust.refreshFrom(cu, force)
}

// requiresMachine returns whether specified job requires a specific machine.
func (eg *Engine) requiresMachine(j *job.Job) ([]string, bool) {
	requirements := j.Requirements()

	bootID, ok := requirements[job.FleetXConditionMachineBootID]
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

		machineBootIDs := make([]string, len(machines))
		for i, mach := range machines {
			machineBootIDs[i] = mach.BootID
		}
		sort.Strings(machineBootIDs)
		return machineBootIDs, nil
	}

	// this is usually a cheap no-op
	eg.refreshCluster(false)

	// as an initial heuristic, choose the k least loaded, with k = partitionSize
	machineBootIDs := eg.clust.kLeastLoaded(partitionSize)

	sort.Strings(machineBootIDs)
	return machineBootIDs, nil
}
