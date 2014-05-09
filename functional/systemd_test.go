package functional

import (
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/systemd"
	"github.com/coreos/fleet/unit"
)

func TestSystemdUnitFlow(t *testing.T) {
	uDir, err := ioutil.TempDir("", "fleet-")
	if err != nil {
		t.Fatalf("Failed creating tempdir: %v", err)
	}
	defer os.RemoveAll(uDir)

	mach := machine.New(machine.MachineState{ID: "XXX"})
	mgr, err := systemd.NewSystemdManager(mach, uDir)
	if err != nil {
		t.Fatalf("Failed initializing SystemdManager: %v", err)
	}

	units, err := mgr.Units()
	if err != nil {
		t.Fatalf("Failed calling Units(): %v", err)
	}

	if len(units) > 0 {
		t.Fatalf("Expected no units to be returned, got %v", units)
	}

	uf := unit.NewUnit(`[Service]
ExecStart=/usr/bin/sleep 3000
`)
	j := job.NewJob("hello.service", *uf)

	if err := mgr.LoadJob(j); err != nil {
		t.Fatalf("Failed loading job: %v", err)
	}

	units, err = mgr.Units()
	if err != nil {
		t.Fatalf("Failed calling Units(): %v", err)
	}

	if !reflect.DeepEqual([]string{"hello.service"}, units){
		t.Fatalf("Expected [hello.service], got %v", units)
	}

	mgr.UnloadJob("hello.service")

	units, err = mgr.Units()
	if err != nil {
		t.Fatalf("Failed calling Units(): %v", err)
	}

	if len(units) > 0 {
		t.Fatalf("Expected no units to be returned, got %v", units)
	}
}
