package registry

import (
	"reflect"
	"testing"

	"github.com/coreos/fleet/etcd"
	"github.com/coreos/fleet/unit"
)

type action struct {
	key string
	val string
	rec bool
}

type testEtcdClient struct {
	sets    []action
	deletes []action
}

func (t *testEtcdClient) Do(req etcd.Action) (*etcd.Result, error) {
	if s, ok := req.(*etcd.Set); ok {
		t.sets = append(t.sets, action{key: s.Key, val: s.Value})
	} else if d, ok := req.(*etcd.Delete); ok {
		t.deletes = append(t.deletes, action{key: d.Key, rec: d.Recursive})
	}
	return nil, nil
}
func (t *testEtcdClient) Wait(req etcd.Action, ch <-chan bool) (*etcd.Result, error) {
	return nil, nil
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
	r.RemoveUnitState(j)
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
}
