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
	err     error
	sets    []action
	deletes []action
}

func (t *testEtcdClient) Do(req etcd.Action) (*etcd.Result, error) {
	if s, ok := req.(*etcd.Set); ok {
		t.sets = append(t.sets, action{key: s.Key, val: s.Value})
	} else if d, ok := req.(*etcd.Delete); ok {
		t.deletes = append(t.deletes, action{key: d.Key, rec: d.Recursive})
	}
	return nil, t.err
}
func (t *testEtcdClient) Wait(req etcd.Action, ch <-chan bool) (*etcd.Result, error) {
	return nil, t.err
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

	r.SaveUnitState(j, us)

	json := `{"loadState":"abc","activeState":"def","subState":"ghi","machineState":{"ID":"mymachine","PublicIP":"","Metadata":null,"Version":"","TotalResources":{"Cores":0,"Memory":0,"Disk":0},"FreeResources":{"Cores":0,"Memory":0,"Disk":0},"LoadedUnits":0}}`
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
			in:   &unit.UnitState{"foo", "bar", "baz", ""},
			want: &unitStateModel{"foo", "bar", "baz", nil},
		},
		{
			in:   &unit.UnitState{"foo", "bar", "baz", "woof"},
			want: &unitStateModel{"foo", "bar", "baz", &machine.MachineState{ID: "woof"}},
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
			in: &unitStateModel{"foo", "bar", "baz", nil},
			want: &unit.UnitState{
				LoadState:   "foo",
				ActiveState: "bar",
				SubState:    "baz",
				MachineID:   "",
			},
		},
		{
			in: &unitStateModel{"z", "x", "y", &machine.MachineState{ID: "abcd"}},
			want: &unit.UnitState{
				LoadState:   "z",
				ActiveState: "x",
				SubState:    "y",
				MachineID:   "abcd",
			},
		},
	} {
		got := modelToUnitState(tt.in)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("case %d: got %#v, want %#v", i, got, tt.want)
		}
	}
}
