package rpc

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/systemd"
	"github.com/coreos/fleet/unit"
)

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
	mgr, err := systemd.NewSystemdUnitManager(uDir)
	if err != nil {
		t.Fatalf("unexpected error creating systemd unit manager: %v", err)
	}

	mach := machine.NewCoreOSMachine(*state, mgr)
	e := &testEtcdKeysAPI{}
	etcdReg := &EtcdRegistry{kAPI: e, keyPrefix: "/fleet/"}

	reg := NewRegistryMux(etcdReg, mach)

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
