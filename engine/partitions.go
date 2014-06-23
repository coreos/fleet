package engine

import (
	"sort"
	"sync"

	"github.com/coreos/fleet/job"
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

func (clust *cluster) machineCreated(machID string) {
	clust.mutex.Lock()
	defer clust.mutex.Unlock()

	if !clust.upToDate {
		return
	}

	clust.populateMachine(machID)
}

func (clust *cluster) machineRemoved(machID string) {
	clust.mutex.Lock()
	defer clust.mutex.Unlock()

	if !clust.upToDate {
		return
	}

	clust.deleteMachine(machID)
}

// jobScheduled handles the job scheduled event
func (clust *cluster) jobScheduled(jobName, target string) {
	clust.mutex.Lock()
	defer clust.mutex.Unlock()

	if !clust.upToDate {
		return
	}

	clust.populateJob(jobName, target)
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

func (clust *cluster) populateJob(jobName string, machineID string) {
	clust.jobsToMachines[jobName] = machineID
	clust.machineJobCount[machineID] = clust.machineJobCount[machineID] + 1
}

func (clust *cluster) deleteJob(jobName string) {
	machineID, ok := clust.jobsToMachines[jobName]

	// TODO(uwedeportivo): this might be a signal that a refresh is needed
	if !ok {
		return
	}

	clust.machineJobCount[machineID] = clust.machineJobCount[machineID] - 1
	delete(clust.jobsToMachines, jobName)
}

func (clust *cluster) populateMachine(machID string) {
	clust.machineJobCount[machID] = 1
}

func (clust *cluster) deleteMachine(machID string) {
	delete(clust.machineJobCount, machID)
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

	ms, _ := eg.registry.Machines()
	for _, m := range ms {
		cu.populateMachine(m.ID)
	}

	jobs, _ := eg.registry.Jobs()
	for _, j := range jobs {
		tgt, _ := eg.registry.JobTarget(j.Name)
		if tgt != "" {
			cu.populateJob(j.Name, tgt)
		}
	}

	eg.clust.refreshFrom(cu, force)
}

// partitionCluster returns a slice of IDs from a subset of active machines
// that should be considered for scheduling the specified job.
// The returned slice is sorted by ascending lexicographical string value of machine boot id.
func (eg *Engine) partitionCluster(j *job.Job) ([]string, error) {
	if machID, ok := j.RequiredTarget(); ok {
		return []string{machID}, nil
	}

	// TODO(uwedeportivo): for now punt on jobs with requirements and offer to all machines
	// because agents are decoding the requirements
	if len(j.Requirements()) > 0 {
		machines, _ := eg.registry.Machines()

		machineIDs := make([]string, len(machines))
		for i, mach := range machines {
			machineIDs[i] = mach.ID
		}
		sort.Strings(machineIDs)
		return machineIDs, nil
	}

	// this is usually a cheap no-op
	eg.refreshCluster(false)

	// as an initial heuristic, choose the k least loaded, with k = partitionSize
	machineIDs := eg.clust.kLeastLoaded(partitionSize)

	sort.Strings(machineIDs)
	return machineIDs, nil
}
