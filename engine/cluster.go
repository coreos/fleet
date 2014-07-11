package engine

import (
	"sort"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/resource"
)

const (
	partitionSize = 5
)

// ClusterCache encapsulates the state of the cluster (in particular, things
// like the MachineState of every agent) in order for the fleet engine to be
// able to make scheduling decisions based on the overall state of the cluster.
// It is a point-in-time snapshot and makes no effort internally to keep up to
// date with the actual state of the cluster; it must be updated by external
// users as necessary.
type ClusterCache interface {
	// TrackMachine adds the given machine to the cluster view.
	TrackMachine(m *machine.MachineState)

	// MachinePresent determines if the referenced Machine appears to be a
	// current member of the cluster.
	MachinePresent(machID string) bool

	// Candidates returns a slice of zero or more machine IDs that should be
	// considered for scheduling the specified job. The returned slice is
	// sorted by ascending lexicographical string value.
	Candidates(j *job.Job) []string

	// AddJob adds the given Job to the cluster view; it is intended to be
	// used during the engine's reconcilation process to ensure that Job
	// scheduling decisions are reflected in the state of the cluster.
	AddJob(machID string, j *job.Job)
}

// cluster is the canonical ClusterCache implementation providing basic
// resource-based scheduling. cluster is NOT thread-safe.
type cluster struct {
	machines map[string]*machine.MachineState
}

func newCluster() ClusterCache {
	return &cluster{
		machines: make(map[string]*machine.MachineState),
	}
}

func (c *cluster) TrackMachine(m *machine.MachineState) {
	c.machines[m.ID] = m
}

func (c *cluster) MachinePresent(machID string) bool {
	_, ok := c.machines[machID]
	return ok
}

// AddJob updates the FreeResources of the MachineState by the given
// machine ID by subtracting the resource reservation of the given Job and
// incrementing the LoadedUnits count
func (c *cluster) AddJob(machID string, j *job.Job) {
	old := c.machines[machID].FreeResources
	c.machines[machID].FreeResources = resource.Sub(old, j.Resources())
	c.machines[machID].LoadedUnits++
}

// Candidates determines which machines are eligible to run the given Job,
// based on resource requirements and the current load in the cluster
func (c *cluster) Candidates(j *job.Job) []string {
	if machID, ok := j.RequiredTarget(); ok {
		return []string{machID}
	}

	var machineIDs []string
	if j.Resources().Empty() {
		machineIDs = c.kLeastLoaded(partitionSize)
	} else {
		machineIDs = c.sufficientResources(j.Resources())
	}
	sort.Strings(machineIDs)
	return machineIDs
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
