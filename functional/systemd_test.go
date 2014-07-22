package functional

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"reflect"
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/systemd"
	"github.com/coreos/fleet/unit"
)

func TestSystemdUnitFlow(t *testing.T) {
	uDir, err := ioutil.TempDir("", "fleet-")
	if err != nil {
		t.Fatalf("Failed creating tempdir: %v", err)
	}
	defer os.RemoveAll(uDir)

	mgr, err := systemd.NewSystemdUnitManager(uDir)
	if err != nil {
		t.Fatalf("Failed initializing SystemdUnitManager: %v", err)
	}

	units, err := mgr.Units()
	if err != nil {
		t.Fatalf("Failed calling Units(): %v", err)
	}

	if len(units) > 0 {
		t.Fatalf("Expected no units to be returned, got %v", units)
	}

	contents := `[Service]
ExecStart=/usr/bin/sleep 3000
`
	name := fmt.Sprintf("fleet-unit-%d.service", rand.Int63())
	uf, err := unit.NewUnit(contents)
	if err != nil {
		t.Fatalf("Invalid unit file: %v", err)
	}
	hash := uf.Hash().String()
	j := job.NewJob(name, *uf)

	if err := mgr.Load(j.Name, j.Unit); err != nil {
		t.Fatalf("Failed loading job: %v", err)
	}

	units, err = mgr.Units()
	if err != nil {
		t.Fatalf("Failed calling Units(): %v", err)
	}

	if !reflect.DeepEqual([]string{name}, units) {
		t.Fatalf("Expected [hello.service], got %v", units)
	}

	us, err := mgr.GetUnitState(name)
	if err == nil {
		expect := unit.UnitState{"loaded", "inactive", "dead", "", hash}
		if !reflect.DeepEqual(expect, *us) {
			t.Errorf("Expected UnitState %v, got %v", expect, *us)
		}
	} else {
		t.Errorf("Failed determining unit state: %v", err)
	}

	mgr.Start(name)

	us, err = mgr.GetUnitState(name)
	if err == nil {
		expect := unit.UnitState{"loaded", "active", "running", "", hash}
		if !reflect.DeepEqual(expect, *us) {
			t.Errorf("Expected UnitState %v, got %v", expect, *us)
		}
	} else {
		t.Errorf("Failed determining unit state: %v", err)
	}

	mgr.Stop(name)

	us, err = mgr.GetUnitState(name)
	if err == nil {
		expect := unit.UnitState{"loaded", "inactive", "dead", "", hash}
		if !reflect.DeepEqual(expect, *us) {
			t.Errorf("Expected UnitState %v, got %v", expect, *us)
		}
	} else {
		t.Errorf("Failed determining unit state: %v", err)
	}

	mgr.Unload(name)

	units, err = mgr.Units()
	if err != nil {
		t.Fatalf("Failed calling Units(): %v", err)
	}

	if len(units) > 0 {
		t.Fatalf("Expected no units to be returned, got %v", units)
	}
}
