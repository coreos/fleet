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

type machineStates []*machine.MachineState

func (m machineStates) Len() int      { return len(m) }
func (m machineStates) Swap(i, j int) { m[i], m[j] = m[j], m[i] }

// byUnitCount embeds machineStates and implements sort.Interface to sort MachineStates by their fleet LoadedUnit count
type byUnitCount struct{ machineStates }

func (m byUnitCount) Less(i, j int) bool {
	return m.machineStates[i].LoadedUnits < m.machineStates[j].LoadedUnits
}

// byFreeResources embeds machineStates and implements sort.Interface to sort MachineStates by their FreeResources
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

// cluster encapsulates the state of the cluster (in particular, agent
// resource availability) in order for the engine to make resource-based
// scheduling decisions
type cluster struct {
	machines map[string]*machine.MachineState
}

func newCluster() *cluster {
	return &cluster{
		machines: make(map[string]*machine.MachineState),
	}
}

// trackMachine adds the given machine to the cluster view
func (c *cluster) trackMachine(m *machine.MachineState) {
	c.machines[m.ID] = m
}

// machinePresent determines if the referenced Machine appears to be a
// current member of the cluster based on the local cache
func (c *cluster) machinePresent(machID string) bool {
	_, ok := c.machines[machID]
	return ok
}

// addJobToMachine updates the FreeResources of the MachineState by the given
// machine ID by subtracting the resource reservation of the given Job. It is
// intended to be called during the engine's reconcilation process after a
// Job has been scheduled in order to ensure subsequent scheduling decisions
// factor this in
func (c *cluster) addJobToMachine(machID string, j *job.Job) {
	old := c.machines[machID].FreeResources
	c.machines[machID].FreeResources = resource.Sub(old, j.Resources())
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

// haveResources returns a slice containing a machine ID representing a
// machine with free resources greater than the given requirement, or an empty
// slice if none exists
func (c *cluster) haveResources(req resource.ResourceTuple) []string {
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

// partition returns a slice of machine IDs from a subset of active machines that
// should be considered for scheduling the specified job. The returned slice
// is sorted by ascending lexicographical string value.
func (c *cluster) partition(j *job.Job) []string {
	if machID, ok := j.RequiredTarget(); ok {
		return []string{machID}
	}

	var machineIDs []string
	if j.Resources().Empty() {
		machineIDs = c.kLeastLoaded(partitionSize)
	} else {
		machineIDs = c.haveResources(j.Resources())
	}
	sort.Strings(machineIDs)
	return machineIDs
}
