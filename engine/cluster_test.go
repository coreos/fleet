package engine

import (
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/resource"
	"github.com/coreos/fleet/unit"
)

var unitcount = map[string]int{
	"empty":        0,
	"small":        1,
	"medium":       5,
	"large":        10,
	"huge":         100,
	"preposterous": 1000,
}

func TestByUnitCount(t *testing.T) {
	ms := machineStates{}
	for id, count := range unitcount {
		ms = append(ms, &machine.MachineState{ID: id, LoadedUnits: count})
	}
	// To ensure we're not totally relying on the random map iteration order, inject one at the end
	ms = append(ms, &machine.MachineState{ID: "deuxzero", LoadedUnits: 20})
	sort.Sort(byUnitCount{ms})
	sorted := []string{"empty", "small", "medium", "large", "deuxzero", "huge", "preposterous"}
	for i, m := range ms {
		want := sorted[i]
		got := m.ID
		if got != want {
			t.Fatalf("byUnitCount: got %v, want %v", got, want)
		}
	}
}

var freeresources = map[string]resource.ResourceTuple{
	"empty": resource.ResourceTuple{0, 0, 0},
	"small": resource.ResourceTuple{1000, 4 * 1024, 64 * 1024},
	"huge":  resource.ResourceTuple{10000, 32 * 1024, 1024 * 1024},
}

func TestByResources(t *testing.T) {
	ms := machineStates{}
	for id, res := range freeresources {
		ms = append(ms, &machine.MachineState{ID: id, FreeResources: res})
	}
	// To ensure we're not totally relying on the random map iteration order, inject one at the end
	ms = append(ms, &machine.MachineState{ID: "medium", FreeResources: resource.ResourceTuple{5000, 16 * 1024, 128 * 1024}})
	sort.Sort(byFreeResources{ms})
	sorted := []string{"empty", "small", "medium", "huge"}
	for i, m := range ms {
		want := sorted[i]
		got := m.ID
		if got != want {
			t.Fatalf("byResources: got %v, want %v", got, want)
		}
	}
}

func newEmptyTestCluster() *cluster {
	c := newCluster()
	return c.(*cluster)
}

func newTestCluster() *cluster {
	c := &cluster{
		machines: make(map[string]*machine.MachineState),
	}
	var ms []*machine.MachineState
	for id, count := range unitcount {
		ms = append(ms, &machine.MachineState{
			ID:            id,
			LoadedUnits:   count,
			FreeResources: freeresources[id],
		})
	}
	for _, m := range ms {
		c.TrackMachine(m)
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

func TestHaveResources(t *testing.T) {
	c := newTestCluster()
	want := []string{"small"}
	got := c.sufficientResources(resource.ResourceTuple{100, 2 * 1024, 32 * 1024})
	if !reflect.DeepEqual(got, want) {
		t.Errorf("haveResources: got %v, want %v", got, want)
	}
}

func TestCandidates(t *testing.T) {
	for i, tt := range []struct {
		c        *cluster
		contents string
		want     []Decision
	}{
		{
			// empty cluster = no results
			&cluster{},
			``,
			nil,
		},
		/*
			// TODO(jonboulle): fix me, obviously
			{
				// should be limited to partitionSize=5, sorted lexigraphically
				newTestCluster(),
				``,
				[]string{"empty", "huge", "large", "medium", "small"},
			},
		*/
		{
			// specific MachineID should return only that one
			newTestCluster(),
			"[X-Fleet]\nX-ConditionMachineID=large",
			[]Decision{Decision{Name: "foo", Machine: "large"}},
		},
	} {
		u, err := unit.NewUnit(tt.contents)
		if err != nil {
			t.Fatalf("case %d: unable to create NewUnit: %v", i, err)
		}
		j := job.NewJob("foo", *u)
		got := tt.c.Decisions([]*job.Job{j})
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("case %d: got %s, want %s", i, got, tt.want)
		}
	}
}

func TestTrackJobPeers(t *testing.T) {
	c := newEmptyTestCluster()

	// Add a single Job with peers
	c.trackJobPeers("foo", []string{"bar", "baz"})

	want := map[string][]string{
		"foo": []string{"bar", "baz"},
		"bar": []string{"foo"},
		"baz": []string{"foo"},
	}

	got := c.jobPeers
	if !reflect.DeepEqual(got, want) {
		t.Errorf("initial trackJobPeers failed got %v, want %v", got, want)
	}

	// Add a separate Job with an existing common peer
	c.trackJobPeers("woof", []string{"quack", "foo"})

	want = map[string][]string{
		"foo":   []string{"bar", "baz", "woof"},
		"bar":   []string{"foo"},
		"baz":   []string{"foo"},
		"woof":  []string{"quack", "foo"},
		"quack": []string{"woof"},
	}

	got = c.jobPeers
	if !reflect.DeepEqual(got, want) {
		t.Errorf("second trackJobPeers failed got %v, want %v", got, want)
	}

	c.trackJobPeers("baz", []string{"quack", "meow"})

	want = map[string][]string{
		"foo":   []string{"bar", "baz", "woof"},
		"bar":   []string{"foo"},
		"baz":   []string{"foo", "quack", "meow"},
		"woof":  []string{"quack", "foo"},
		"quack": []string{"woof", "baz"},
		"meow":  []string{"baz"},
	}

	got = c.jobPeers
	if !reflect.DeepEqual(got, want) {
		t.Errorf("final trackJobPeers failed got %v, want %v", got, want)
	}
}

func TestTrackJobConflicts(t *testing.T) {
	c := newEmptyTestCluster()
	c.trackJobConflicts("foo", []string{"bar", "baz"})
	want := []string{"bar", "baz"}
	got := c.jobConflicts["foo"]
	if !reflect.DeepEqual(got, want) {
		t.Errorf("trackJobConflicts: got %v, want %v", got, want)
	}

}

func newTestJobWithXFleet(t *testing.T, name, machine, metadata string) *job.Job {
	contents := fmt.Sprintf(`
[X-Fleet]
%s
`, metadata)
	u, err := unit.NewUnit(contents)
	if err != nil {
		t.Fatalf("error creating Unit from %q: %v", contents, err)
	}
	j := job.NewJob(name, *u)
	if j == nil {
		t.Fatalf("error creating Job %q from %q", name, u)
	}
	if machine != "" {
		j.TargetMachineID = machine
	}
	return j
}

func TestTrackJob(t *testing.T) {
	c := newEmptyTestCluster()
	j1 := newTestJobWithXFleet(t, "j1", "m1", `XConditionMachineOf=j2`)
	j2 := newTestJobWithXFleet(t, "j2", "m1", ``)
	j3 := newTestJobWithXFleet(t, "j3", "m2", `XConflicts=j4`)
	j4 := newTestJobWithXFleet(t, "j4", `m3`, ``)

	c.TrackJob(j1)
	c.TrackJob(j2)
	c.TrackJob(j3)
	c.TrackJob(j4)

	wantm := map[string][]string{
		"m1": []string{"j1", "j2"},
		"m2": []string{"j3"},
		"m3": []string{"j4"},
	}
	gotm := c.machsToJobs
	if !reflect.DeepEqual(gotm, wantm) {
		t.Errorf("bad machsToJobs: got %v, want %v", gotm, wantm)
	}

	wantj := map[string]string{
		"j1": "m1",
		"j2": "m1",
		"j3": "m2",
		"j4": "m3",
	}
	gotj := c.jobToMach
	if !reflect.DeepEqual(gotj, wantj) {
		t.Errorf("bad jobToMach: got %v, want %v", gotj, wantj)
	}
}

func TestMachinePresent(t *testing.T) {
	c := newEmptyTestCluster()
	if c.MachinePresent("miyagi") {
		t.Errorf("MachinePresent returned true for no machines?!")
	}
	c.TrackMachine(&machine.MachineState{ID: "miyagi"})
	if !c.MachinePresent("miyagi") {
		t.Errorf("MachinePresent returned false unexpectedly")
	}
	if c.MachinePresent("daniel") {
		t.Errorf("MachinePresent returned true unexpectedly")
	}
}

func TestResolvePeers(t *testing.T) {
	c := &cluster{
		jobPeers: map[string][]string{
			"foo":   {"bar", "baz", "woof"},
			"bar":   {"foo"},
			"baz":   {"foo", "quack", "meow"},
			"woof":  {"quack", "foo"},
			"quack": {"woof", "baz"},
			"meow":  {"baz"},
			"bark":  {"yap"},
			"yap":   {"bark"},
		},
	}
	for i, tt := range []struct {
		jName string
		want  []string
	}{
		{
			"foo",
			[]string{"bar", "baz", "foo", "meow", "quack", "woof"},
		},
		{
			"quack",
			[]string{"bar", "baz", "foo", "meow", "quack", "woof"},
		},
		{
			"bark",
			[]string{"bark", "yap"},
		},
		{
			"yap",
			[]string{"bark", "yap"},
		},
	} {
		got := c.resolvePeers(tt.jName)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("case %d: got %v, want %v", i, got, tt.want)
		}
	}

}

func TestContains(t *testing.T) {
	for i, tt := range []struct {
		list []string
		test string
		want bool
	}{
		{[]string{"abc"}, "abc", true},
		{[]string{"abc", "def"}, "abc", true},
		{[]string{"foo", "bar"}, "bar", true},
		{[]string{"foo", "bar"}, "baz", false},
		{[]string{}, "something", false},
		{[]string{}, "", false},
	} {
		got := contains(tt.list, tt.test)
		if got != tt.want {
			t.Errorf("case %d: got %t, want %t", i, got, tt.want)
		}
	}
}

func TestPartitionPods(t *testing.T) {
	foo := newTestJobWithXFleet(t, "foo", "", "X-ConditionMachineOf=bar")
	bar := newTestJobWithXFleet(t, "bar", "", "X-ConditionMachineOf=baz")
	baz := newTestJobWithXFleet(t, "baz", "", "X-Conflicts=foo")
	c := newEmptyTestCluster()
	c.TrackJob(foo)
	c.TrackJob(bar)
	/*
		c := &cluster{
			jobPeers: map[string][]string{
				"foo":   {"bar", "baz", "woof"},
				"bar":   {"foo"},
				"baz":   {"foo", "quack", "meow"},
				"woof":  {"quack", "foo"},
				"quack": {"woof", "baz"},
				"meow":  {"baz"},
				"bark":  {"yap"},
				"yap":   {"bark"},
			},
		}
	*/
	for i, tt := range []struct {
		jm   map[string]*job.Job
		pods []*pod
		decs []Decision
	}{
		{
			map[string]*job.Job{"foo": foo, "bar": bar, "baz": baz},
			nil,
			[]Decision{Decision{Name: "foo"}, Decision{Name: "bar"}, Decision{Name: "baz"}},
		},
	} {
		p, d := c.partitionPods(tt.jm)
		if !reflect.DeepEqual(p, tt.pods) {
			t.Errorf("case %d: bad pods returned - got %v, want %v", i, p, tt.pods)
		}
		for _, want := range tt.decs {
			var found bool
			for _, got := range d {
				if got.Name == want.Name {
					found = true
				}
			}
			if !found {
				t.Errorf("case %d: did not find expecte Decision for %v", i, want)
			}
		}
	}
}
