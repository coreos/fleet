package engine

import (
	"fmt"
	"sort"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/resource"
)

const (
	partitionSize = 5
)

// ClusterCache encapsulates the state of the cluster (in particular, things
// like the MachineState of every agent) in order to make scheduling decisions
// based on the overall state of the cluster.  It is a point-in-time snapshot
// and makes no effort internally to keep up to date with the actual state of
// the cluster; it must be updated by external users as necessary. In
// particular, the TrackMachine and TrackJob functions should be use to
// initialize the ClusterCache.
type ClusterCache interface {
	// TrackMachine adds the given machine to the cluster view.
	TrackMachine(m *machine.MachineState)

	// TrackJob adds the given active Job to the cluster view, tracking its
	// currently scheduled location.
	TrackJob(j *job.Job)

	// MachinePresent determines if the referenced Machine appears to be a
	// current member of the cluster.
	MachinePresent(machID string) bool

	// Decisions returns a list of zero or more scheduling decisions.
	Decisions(jobs []*job.Job) []Decision
}

// Decision represents a scheduling Decision made by a Cluster Cache, mapping a
// Job to a Machine. A scheduling Decision is either successful (non-nil
// Machine, nil Reason) or unsuccessful (nil Machine, non-nil Reason).
type Decision struct {
	// Job name
	Name string
	// MachineID of the machine to which the job should be scheduled. nil if Reason is set.
	Machine string
	// Reason the Job is unable to be scheduled.
	Reason error
}

// cluster is the canonical ClusterCache implementation providing basic
// resource-based scheduling. cluster is NOT thread-safe.
type cluster struct {
	machines     map[string]*machine.MachineState
	machsToJobs  map[string][]string
	jobToMach    map[string]string
	jobConflicts map[string][]string
	jobPeers     map[string][]string
}

// pod represents a group of Jobs with Peer requirements, and aggregates the
// conflicts and resources of the constituent jobs
type pod struct {
	jobs []*job.Job
}

func (p *pod) conflicts() (c []string) {
	for _, j := range p.jobs {
		c = append(c, j.Peers()...)
	}
	return
}

func (p *pod) resources() (r resource.ResourceTuple) {
	for _, j := range p.jobs {
		r = resource.Sum(r, j.Resources())
	}
	return
}

func newCluster() ClusterCache {
	return &cluster{
		machines:     make(map[string]*machine.MachineState),
		machsToJobs:  make(map[string][]string),
		jobToMach:    make(map[string]string),
		jobConflicts: make(map[string][]string),
		jobPeers:     make(map[string][]string),
	}
}

func (c *cluster) TrackMachine(m *machine.MachineState) {
	c.machines[m.ID] = m
}

func (c *cluster) TrackJob(j *job.Job) {
	mID := j.TargetMachineID
	m := c.machsToJobs[mID]
	if m == nil {
		m = make([]string, 0)
	}
	m = append(m, j.Name)
	c.machsToJobs[mID] = m
	c.jobToMach[j.Name] = mID
	// Since this Job is already scheduled to a machine, its resources are
	// captured by that machine's FreeResources, so no need to track them
	// here. However, we do need to track its peers and conflicts for other
	// scheduling decisions.
	c.trackJobPeers(j.Name, j.Peers())
	c.trackJobConflicts(j.Name, j.Conflicts())
}

func (c *cluster) MachinePresent(machID string) bool {
	_, ok := c.machines[machID]
	return ok
}

// scheduleJob updates the FreeResources of the MachineState by the given
// machine ID by subtracting the resource reservation of the given Job and
// incrementing the LoadedUnits count
func (c *cluster) scheduleJob(machID string, j *job.Job) {
	old := c.machines[machID].FreeResources
	c.machines[machID].FreeResources = resource.Sub(old, j.Resources())
	c.machines[machID].LoadedUnits++
}

// Decisions determines which machines are eligible to run the given Job(s),
// based on job requirements, resource requirements, and the current load in
// the cluster
func (c *cluster) Decisions(jobs []*job.Job) (decs []Decision) {
	jobMap := make(map[string]*job.Job)
	for _, j := range jobs {
		if machID, ok := j.RequiredTarget(); ok {
			// TODO(jonboulle): should we care about resource requirements and other restrictions in this case?
			decs = append(decs, Decision{
				Name:    j.Name,
				Machine: machID,
			})
			continue
		}
		jobMap[j.Name] = j
	}

	// Partition jobs into groups of peers (aka "pods").
	pods, failures := c.partitionPods(jobMap)
	for _, f := range failures {
		decs = append(decs, f)
	}

	for _, p := range pods {
		if len(p.jobs) > 0 {
		}
		/*
				if len(j.Conflicts()) > 0 {
					// TODO(jonboulle): deal with this. including transitive conflicts.
					return []string{}
				}
			var machineIDs []string
			if j.Resources().Empty() {
				machineIDs = c.kLeastLoaded(partitionSize)
			} else {
				machineIDs = c.sufficientResources(j.Resources())
			}
			sort.Strings(machineIDs)
		*/
	}
	return
}

// machineStates is used by cluster to order the machines in the cluster
// view by different criteria (for example, free resources)
type machineStates []*machine.MachineState

func (m machineStates) Len() int      { return len(m) }
func (m machineStates) Swap(i, j int) { m[i], m[j] = m[j], m[i] }

// byUnitCount embeds machineStates and implements sort.Interface to sort
// MachineStates by their fleet LoadedUnit count
type byUnitCount struct{ machineStates }

func (m byUnitCount) Less(i, j int) bool {
	return m.machineStates[i].LoadedUnits < m.machineStates[j].LoadedUnits
}

// byFreeResources embeds machineStates and implements sort.Interface to sort
// MachineStates by their FreeResources
type byFreeResources struct{ machineStates }

func (m byFreeResources) Less(i, j int) bool {
	ifr := m.machineStates[i].FreeResources
	jfr := m.machineStates[j].FreeResources
	// Currently this orders somewhat naively, not always considering all
	// three dimensions. This isn't a huge deal because when making the
	// actual scheduling decision we ensure that all dimensions are
	// satisfied exactly, but it may be worth revisiting at a later time
	return ifr.Cores < jfr.Cores && ifr.Memory < jfr.Memory && ifr.Disk < jfr.Disk
}

// kLeastLoaded returns a list of machine IDs representing the k machines
// with the lowest number of loaded units
func (c *cluster) kLeastLoaded(k int) []string {
	ms := machineStates{}
	for _, m := range c.machines {
		ms = append(ms, m)
	}

	sort.Sort(byUnitCount{ms})
	j := k
	if len(ms) < k {
		j = len(ms)
	}
	least := make([]string, 0)
	for i := 0; i < j; i++ {
		least = append(least, ms[i].ID)
	}
	return least
}

// sufficientResources returns a slice containing a machine ID representing a
// machine with free resources greater than the given requirement, or an empty
// slice if none exists
func (c *cluster) sufficientResources(req resource.ResourceTuple) []string {
	ms := machineStates{}
	for _, m := range c.machines {
		ms = append(ms, m)
	}

	sort.Sort(byFreeResources{ms})
	j := sort.Search(len(ms), func(i int) bool {
		r := ms[i].FreeResources
		return r.Cores >= req.Cores && r.Memory >= req.Memory && r.Disk >= req.Disk
	})
	if j >= len(ms) {
		// No suitable candidates
		return []string{}
	}
	return []string{ms[j].ID}
}

// trackJobConflicts stores the conflicts of a Job
func (c *cluster) trackJobConflicts(jName string, conflicts []string) {
	c.jobConflicts[jName] = conflicts
}

// trackJobPeers stores the bi-directional dependency between a Job
// and its peers:
//    1 Job --> N Peers
//    N Peers --> 1 Job
func (c *cluster) trackJobPeers(jName string, peers []string) {
	if _, ok := c.jobPeers[jName]; !ok {
		c.jobPeers[jName] = make([]string, 0)

	}
	c.jobPeers[jName] = append(c.jobPeers[jName], peers...)

	for _, peer := range peers {
		if _, ok := c.jobPeers[peer]; !ok {
			c.jobPeers[peer] = make([]string, 0)
		}
		c.jobPeers[peer] = append(c.jobPeers[peer], jName)
	}
}

// resolvePeers determines the transitive closure of all Peers of the
// Job of a given Name, sorted lexicographically
func (c *cluster) resolvePeers(jName string) []string {
	// TODO(jonboulle): use pkg.Set once merged
	seen := make(map[string]bool)
	all := make([]string, 0)
	c.resolve(jName, seen)
	for j, _ := range seen {
		all = append(all, j)
	}
	sort.Strings(all)
	return all
}

func (c *cluster) resolve(name string, seen map[string]bool) {
	if _, ok := seen[name]; ok {
		return
	}
	seen[name] = true
	for _, peer := range c.jobPeers[name] {
		c.resolve(peer, seen)
	}
	return
}

// contains checks if a slice of strings contains the given string
func contains(list []string, str string) bool {
	for _, s := range list {
		if s == str {
			return true
		}
	}
	return false
}

// partitionPods separates the given set of Jobs into pods. Any Jobs resulting
// in unresolvable pods (for example, where the set of Peers overlaps with the
// set of Conflicts) are returned as failed scheduling decisions.
func (c *cluster) partitionPods(jobMap map[string]*job.Job) (pods []*pod, decs []Decision) {
	for jName, j := range jobMap {
		peers := c.resolvePeers(jName)

		var dec *Decision
		for _, p := range peers {
			if contains(j.Conflicts(), p) {
				dec = &Decision{
					Name:   jName,
					Reason: fmt.Errorf("unresolvable conflict: transitive peer %s in Conflicts", p),
				}
			}
		}
		if dec != nil {
			decs = append(decs, *dec)
			continue
		}

		p := &pod{
			jobs: []*job.Job{j},
		}

		for _, peer := range peers {
			p.jobs = append(p.jobs, jobMap[peer])
			delete(jobMap, peer)
		}

		pods = append(pods, p)
	}
	return
}
