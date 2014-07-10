package engine

import (
	"sort"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
)

const (
	partitionSize = 5
)

// machsByUnitCount implements the Sort interfaces to sort a slice of MachineStates by their fleet LoadedUnit count
type machsByUnitCount []*machine.MachineState

func (m machsByUnitCount) Len() int           { return len(m) }
func (m machsByUnitCount) Swap(i, j int)      { m[i], m[j] = m[j], m[i] }
func (m machsByUnitCount) Less(i, j int) bool { return m[i].LoadedUnits < m[j].LoadedUnits }

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

// kLeastLoaded returns a list of machine IDs representing the k machines
// with the lowest number of loaded units
func (c *cluster) kLeastLoaded(k int) []string {
	ms := machsByUnitCount{}
	for _, m := range c.machines {
		ms = append(ms, m)
	}
	sort.Sort(ms)
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

// partition returns a slice of machine IDs from a subset of active machines that
// should be considered for scheduling the specified job. The returned slice
// is sorted by ascending lexicographical string value.
func (c *cluster) partition(j *job.Job) []string {
	if machID, ok := j.RequiredTarget(); ok {
		return []string{machID}
	}

	machineIDs := c.kLeastLoaded(partitionSize)

	sort.Strings(machineIDs)
	return machineIDs
}
