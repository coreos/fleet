package engine

import (
	"reflect"
	"sort"
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/unit"
)

var sizes = map[string]int{
	"empty":        0,
	"small":        1,
	"medium":       5,
	"large":        10,
	"huge":         100,
	"preposterous": 1000,
}

func TestMachsByUnitCount(t *testing.T) {
	ms := machsByUnitCount{}
	for id, count := range sizes {
		ms = append(ms, &machine.MachineState{ID: id, LoadedUnits: count})
	}
	// To ensure we're not totally relying on the random map iteration order, inject one at the end
	ms = append(ms, &machine.MachineState{ID: "deuxzero", LoadedUnits: 20})
	sort.Sort(ms)
	if !sort.IsSorted(ms) {
		t.Errorf("IsSorted(machsByUnitCount) returned false unexpectedly")
	}
	sorted := []string{"empty", "small", "medium", "large", "deuxzero", "huge", "preposterous"}
	for i, m := range ms {
		want := sorted[i]
		got := m.ID
		if got != want {
			t.Fatalf("machsByUnitCount: got %v, want %v", got, want)
		}
	}
}

func newTestCluster() *cluster {
	c := newCluster()
	var ms []*machine.MachineState
	for id, count := range sizes {
		ms = append(ms, &machine.MachineState{ID: id, LoadedUnits: count})
	}
	for _, m := range ms {
		c.machines[m.ID] = m
	}
	return c
}

func TestLeastLoaded(t *testing.T) {
	c := newTestCluster()
	want := []string{"empty", "small", "medium", "large", "huge", "preposterous"}
	got := c.kLeastLoaded(10)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("kLeastLoaded: got %v, want %v", got, want)
	}

	c = newTestCluster()
	want = []string{"empty", "small"}
	got = c.kLeastLoaded(2)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("kLeastLoaded: got %v, want %v", got, want)
	}
}

func TestPartitionCluster(t *testing.T) {
	for i, tt := range []struct {
		c        *cluster
		contents string
		want     []string
	}{
		{
			// empty cluster = no results
			newCluster(),
			``,
			[]string{},
		},
		{
			// should be limited to partitionSize=5, sorted lexigraphically
			newTestCluster(),
			``,
			[]string{"empty", "huge", "large", "medium", "small"},
		},
		{
			// specific MachineID should return only that one
			newTestCluster(),
			"[X-Fleet]\nX-ConditionMachineID=large",
			[]string{"large"},
		},
		{
			// even with no matching ID (in case the machine subsequently comes online)
			newTestCluster(),
			"[X-Fleet]\nX-ConditionMachineID=beer",
			[]string{"beer"},
		},
	} {
		u, err := unit.NewUnit(tt.contents)
		if err != nil {
			t.Fatalf("case %d: unable to create NewUnit: %v", i, err)
		}
		j := job.NewJob("foo", *u)
		got := tt.c.partition(j)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("case %d: got %s, want %s", i, got, tt.want)
		}
	}
}
