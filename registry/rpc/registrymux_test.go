package rpc

import (
	"io/ioutil"
	"os"
	"testing"

	etcd "github.com/coreos/etcd/client"
	"golang.org/x/net/context"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/pkg/lease"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/systemd"
	"github.com/coreos/fleet/unit"
)

type action struct {
	key string
	val string
	rec bool
}

type testEtcdKeysAPI struct {
	etcd.KeysAPI
	gets    []action
	sets    []action
	deletes []action
	res     []*etcd.Response // errors returned from subsequent calls to etcd
	ri      int
	err     []error // results returned from subsequent calls to etcd
	ei      int
}

func (t *testEtcdKeysAPI) Set(_ context.Context, key string, value string, _ *etcd.SetOptions) (*etcd.Response, error) {
	t.sets = append(t.sets, action{key: key, val: value})
	return t.next()
}

func (t *testEtcdKeysAPI) Get(_ context.Context, key string, opts *etcd.GetOptions) (*etcd.Response, error) {
	act := action{key: key}
	if opts != nil {
		act.rec = opts.Recursive
	}
	t.gets = append(t.gets, act)
	return t.next()
}

func (t *testEtcdKeysAPI) Delete(_ context.Context, key string, opts *etcd.DeleteOptions) (*etcd.Response, error) {
	act := action{key: key}
	if opts != nil {
		act.rec = opts.Recursive
	}
	t.deletes = append(t.deletes, act)
	return t.next()
}

func (t *testEtcdKeysAPI) next() (r *etcd.Response, e error) {
	if t.ri < len(t.res) {
		r = t.res[t.ri]
		t.ri++
	}
	if t.ei < len(t.err) {
		e = t.err[t.ei]
		t.ei++
	}
	return r, e
}

func TestRegistryMuxUnitManagement(t *testing.T) {
	uDir, err := ioutil.TempDir("", "fleet-")
	if err != nil {
		t.Fatalf("failed creating tempdir: %v", err)
	}
	defer os.RemoveAll(uDir)

	state := &machine.MachineState{
		ID:       "id",
		PublicIP: "127.0.0.1",
		Metadata: make(map[string]string, 0),
	}
	mgr, err := systemd.NewSystemdUnitManager(uDir, false)
	if err != nil {
		t.Fatalf("unexpected error creating systemd unit manager: %v", err)
	}

	mach := machine.NewCoreOSMachine(*state, mgr)
	e := &testEtcdKeysAPI{}
	etcdReg := registry.NewEtcdRegistry(e, "/fleet/")

	lManager := lease.NewEtcdLeaseManager(e, "/fleet/")
	reg := NewRegistryMux(etcdReg, mach, lManager)

	contents := `
[Unit]
Description = Foo
`
	unitFile, err := unit.NewUnitFile(contents)
	if err != nil {
		t.Fatalf("unexpected error parsing unit %q: %v", contents, err)
	}
	unit := &job.Unit{
		Name:        "foo",
		Unit:        *unitFile,
		TargetState: job.JobStateLoaded,
	}
	if err := reg.CreateUnit(unit); err != nil {
		t.Fatalf("unexpected error creating an unit: %v", err)
	}

	machineID := "testMachine"
	if err := reg.ScheduleUnit(unit.Name, machineID); err != nil {
		t.Fatalf("unexpected error scheduling an unit: %v", err)
	}

	if err := reg.UnscheduleUnit(unit.Name, machineID); err != nil {
		t.Fatalf("unexpected error unscheduling an unit: %v", err)
	}

	if err := reg.DestroyUnit(unit.Name); err != nil {
		t.Fatalf("unexpected error destroying an unit: %v", err)
	}

	if err := reg.RemoveMachineState(machineID); err != nil {
		t.Fatalf("unexpected error removing machine state: %v", err)
	}
}
