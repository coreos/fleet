package registry

import (
	"errors"
	"reflect"
	"testing"

	"github.com/coreos/fleet/etcd"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/unit"
)

type action struct {
	key string
	val string
	rec bool
}

type testEtcdClient struct {
	gets    []action
	sets    []action
	deletes []action
	res     *etcd.Result
	err     error
}

func (t *testEtcdClient) Do(req etcd.Action) (*etcd.Result, error) {
	if s, ok := req.(*etcd.Set); ok {
		t.sets = append(t.sets, action{key: s.Key, val: s.Value})
	} else if d, ok := req.(*etcd.Delete); ok {
		t.deletes = append(t.deletes, action{key: d.Key, rec: d.Recursive})
	} else if g, ok := req.(*etcd.Get); ok {
		t.gets = append(t.gets, action{key: g.Key, rec: g.Recursive})
	}
	return t.res, t.err
}

func (t *testEtcdClient) Wait(req etcd.Action, ch <-chan struct{}) (*etcd.Result, error) {
	return t.res, t.err
}

func TestUnitStatePaths(t *testing.T) {
	r := &EtcdRegistry{nil, "/fleet/"}
	j := "foo.service"
	want := "/fleet/state/foo.service"
	got := r.legacyUnitStatePath(j)
	if got != want {
		t.Errorf("bad unit state path: got %v, want %v", got, want)
	}
	m := "abcdefghij"
	want = "/fleet/states/foo.service/abcdefghij"
	got = r.unitStatePath(m, j)
	if got != want {
		t.Errorf("bad unit state path: got %v, want %v", got, want)
	}
}

func TestSaveUnitState(t *testing.T) {
	e := &testEtcdClient{}
	r := &EtcdRegistry{e, "/fleet/"}
	j := "foo.service"
	mID := "mymachine"
	us := unit.NewUnitState("abc", "def", "ghi", mID)

	// Saving nil unit state should fail
	r.SaveUnitState(j, nil)
	if e.sets != nil || e.deletes != nil {
		t.Logf("sets: %#v", e.sets)
		t.Logf("deletes: %#v", e.deletes)
		t.Fatalf("SaveUnitState of nil state should fail but acted unexpectedly!")
	}

	// Saving unit state with no hash should succeed for now, but should fail
	// in the future. See https://github.com/coreos/fleet/issues/720.
	//r.SaveUnitState(j, us)
	//if len(e.sets) != 1 || e.deletes == nil {
	//	t.Logf("sets: %#v", e.sets)
	//	t.Logf("deletes: %#v", e.deletes)
	//	t.Fatalf("SaveUnitState on UnitState with no hash acted unexpectedly!")
	//}

	us.UnitHash = "quickbrownfox"
	r.SaveUnitState(j, us)

	json := `{"loadState":"abc","activeState":"def","subState":"ghi","machineState":{"ID":"mymachine","PublicIP":"","Metadata":null,"Version":"","TotalResources":{"Cores":0,"Memory":0,"Disk":0},"FreeResources":{"Cores":0,"Memory":0,"Disk":0}},"unitHash":"quickbrownfox"}`
	p1 := "/fleet/state/foo.service"
	p2 := "/fleet/states/foo.service/mymachine"
	want := []action{
		action{key: p1, val: json},
		action{key: p2, val: json},
	}
	got := e.sets
	if !reflect.DeepEqual(got, want) {
		t.Errorf("bad result from SaveUnitState: \ngot\n%#v\nwant\n%#v", got, want)
	}
	if e.deletes != nil {
		t.Errorf("unexpected deletes during SaveUnitState: %#v", e.deletes)
	}
	if e.gets != nil {
		t.Errorf("unexpected gets during RemoveUnitState: %#v", e.gets)
	}
}

func TestRemoveUnitState(t *testing.T) {
	e := &testEtcdClient{}
	r := &EtcdRegistry{e, "/fleet/"}
	j := "foo.service"
	err := r.RemoveUnitState(j)
	if err != nil {
		t.Errorf("unexpected error from RemoveUnitState: %v", err)
	}
	want := []action{
		action{key: "/fleet/state/foo.service", rec: false},
		action{key: "/fleet/states/foo.service", rec: true},
	}
	got := e.deletes
	if !reflect.DeepEqual(got, want) {
		t.Errorf("bad result from RemoveUnitState: \ngot\n%#v\nwant\n%#v", got, want)
	}
	if e.sets != nil {
		t.Errorf("unexpected sets during RemoveUnitState: %#v", e.sets)
	}
	if e.gets != nil {
		t.Errorf("unexpected gets during RemoveUnitState: %#v", e.gets)
	}

	e = &testEtcdClient{err: errors.New("some error")}
	r = &EtcdRegistry{e, "/fleet/"}
	err = r.RemoveUnitState("foo.service")
	if err == nil {
		t.Errorf("did not get expected error from RemoveUnitState")
	}

	e = &testEtcdClient{err: etcd.Error{ErrorCode: etcd.ErrorKeyNotFound}}
	r = &EtcdRegistry{e, "/fleet/"}
	err = r.RemoveUnitState("foo.service")
	if err != nil {
		t.Errorf("unexpected error from RemoveUnitState: %v", err)
	}
}

func TestUnitStateToModel(t *testing.T) {
	for i, tt := range []struct {
		in   *unit.UnitState
		want *unitStateModel
	}{
		{
			in:   nil,
			want: nil,
		},
		{
			// Unit state with no hash and no machineID is OK
			// See https://github.com/coreos/fleet/issues/720
			in:   &unit.UnitState{"foo", "bar", "baz", "", ""},
			want: &unitStateModel{"foo", "bar", "baz", nil, ""},
		},
		{
			// Unit state with hash but no machineID is OK
			in:   &unit.UnitState{"foo", "bar", "baz", "", "heh"},
			want: &unitStateModel{"foo", "bar", "baz", nil, "heh"},
		},
		{
			in:   &unit.UnitState{"foo", "bar", "baz", "woof", "miaow"},
			want: &unitStateModel{"foo", "bar", "baz", &machine.MachineState{ID: "woof"}, "miaow"},
		},
	} {
		got := unitStateToModel(tt.in)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("case %d: got %#v, want %#v", i, got, tt.want)
		}
	}
}

func TestModelToUnitState(t *testing.T) {
	for i, tt := range []struct {
		in   *unitStateModel
		want *unit.UnitState
	}{
		{
			in:   nil,
			want: nil,
		},
		{
			in: &unitStateModel{"foo", "bar", "baz", nil, ""},
			want: &unit.UnitState{
				LoadState:   "foo",
				ActiveState: "bar",
				SubState:    "baz",
				MachineID:   "",
				UnitHash:    "",
			},
		},
		{
			in: &unitStateModel{"z", "x", "y", &machine.MachineState{ID: "abcd"}, ""},
			want: &unit.UnitState{
				LoadState:   "z",
				ActiveState: "x",
				SubState:    "y",
				MachineID:   "abcd",
				UnitHash:    "",
			},
		},
	} {
		got := modelToUnitState(tt.in)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("case %d: got %#v, want %#v", i, got, tt.want)
		}
	}
}

func makeResult(val string) *etcd.Result {
	return &etcd.Result{
		Node: &etcd.Node{
			Value: val,
		},
	}
}

func TestGetUnitState(t *testing.T) {
	for i, tt := range []struct {
		res *etcd.Result // result returned from etcd
		err error        // error returned from etcd
		us  *unit.UnitState
	}{
		{
			// Unit state with no UnitHash should be OK
			res: makeResult(`{"loadState":"abc","activeState":"def","subState":"ghi","machineState":{"ID":"mymachine","PublicIP":"","Metadata":null,"Version":"","TotalResources":{"Cores":0,"Memory":0,"Disk":0},"FreeResources":{"Cores":0,"Memory":0,"Disk":0}}}`),
			err: nil,
			us:  unit.NewUnitState("abc", "def", "ghi", "mymachine"),
		},
		{
			// Unit state with UnitHash should be OK
			res: makeResult(`{"loadState":"abc","activeState":"def","subState":"ghi","machineState":{"ID":"mymachine","PublicIP":"","Metadata":null,"Version":"","TotalResources":{"Cores":0,"Memory":0,"Disk":0},"FreeResources":{"Cores":0,"Memory":0,"Disk":0}},"unitHash":"quickbrownfox"}`),
			err: nil,
			us:  &unit.UnitState{"abc", "def", "ghi", "mymachine", "quickbrownfox"},
		},
		{
			// Unit state with no MachineState should be OK
			res: makeResult(`{"loadState":"abc","activeState":"def","subState":"ghi"}`),
			err: nil,
			us:  &unit.UnitState{"abc", "def", "ghi", "", ""},
		},
		{
			// Bad unit state object should simply result in nil returned
			res: makeResult(`garbage, not good proper json`),
			err: nil,
			us:  nil,
		},
		{
			// Unknown errors should result in nil returned
			res: nil,
			err: errors.New("some random error from etcd"),
			us:  nil,
		},
		{
			// KeyNotFound should result in nil returned
			res: nil,
			err: etcd.Error{ErrorCode: etcd.ErrorKeyNotFound},
			us:  nil,
		},
	} {
		e := &testEtcdClient{
			res: tt.res,
			err: tt.err,
		}
		r := &EtcdRegistry{e, "/fleet/"}
		j := "foo.service"
		us := r.getUnitState(j)
		want := []action{
			action{key: "/fleet/state/foo.service", rec: true},
		}
		got := e.gets
		if !reflect.DeepEqual(got, want) {
			t.Errorf("case %d: bad result from GetUnitState:\ngot\n%#v\nwant\n%#v", i, got, want)
		}
		if !reflect.DeepEqual(us, tt.us) {
			t.Errorf("case %d: bad UnitState:\ngot\n%#v\nwant\n%#v", i, us, tt.us)
		}
	}
}

func usToJson(t *testing.T, us *unit.UnitState) string {
	json, err := marshal(unitStateToModel(us))
	if err != nil {
		t.Fatalf("error marshalling unit: %v", err)
	}
	return json
}

func TestStates(t *testing.T) {
	fus1 := unit.UnitState{"abc", "def", "ghi", "mID1", "xyz"}
	fusn1 := etcd.Node{
		Key:   "/fleet/states/foo/mID1",
		Value: usToJson(t, &fus1),
	}
	fus2 := unit.UnitState{"abc", "def", "ghi", "mID2", "xyz"}
	fusn2 := etcd.Node{
		Key:   "/fleet/states/foo/mID2",
		Value: usToJson(t, &fus2),
	}
	foo := etcd.Node{
		Key: "/fleet/states/foo",
		Nodes: []etcd.Node{
			fusn1,
			fusn2,
		},
	}
	bar1 := etcd.Node{
		Key:   "/fleet/states/bar/asdf",
		Value: ``,
	}
	bar := etcd.Node{
		Key: "/fleet/states/bar",
		Nodes: []etcd.Node{
			bar1,
		},
	}
	res := &etcd.Result{
		Node: &etcd.Node{
			Key: "/fleet/states",
			Nodes: []etcd.Node{
				foo,
				bar,
			},
		},
	}
	e := &testEtcdClient{
		res: res,
	}
	r := &EtcdRegistry{e, "/fleet/"}
	got, err := r.States()
	if err != nil {
		t.Errorf("unexpected error calling States(): %v", err)
	}
	want := []*unit.UnitState{
		&fus1,
		&fus2,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("States() returned unexpected result")
		t.Logf("got:\n%#v", got)
		t.Logf("want:\n%#v", want)
	}

	e = &testEtcdClient{err: etcd.Error{ErrorCode: etcd.ErrorKeyNotFound}}
	r = &EtcdRegistry{e, "/fleet"}
	got, err = r.States()
	if err != nil {
		t.Errorf("unexpected error calling States(): %v", err)
	}
	if len(got) != 0 {
		t.Errorf("States() returned unexpected non-empty result on error: %v", got)
	}

	e = &testEtcdClient{err: errors.New("ur registry don't work")}
	r = &EtcdRegistry{e, "/fleet"}
	got, err = r.States()
	if err == nil {
		t.Errorf("expected error calling States() but got none!")
	}
	if len(got) != 0 {
		t.Errorf("States() returned unexpected non-empty result on error: %v", got)
	}
}
