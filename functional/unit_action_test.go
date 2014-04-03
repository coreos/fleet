package functional

import (
	"strings"
	"testing"

	"github.com/coreos/fleet/functional/platform"
)

func TestUnitRestart(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.DestroyAll()

	if err := cluster.CreateMultiple(1, platform.MachineConfig{}); err != nil {
		t.Fatal(err)
	}
	_, err = waitForNMachines(1)
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := fleetctl("start", "fixtures/units/hello.service"); err != nil {
		t.Fatalf("Unable to start fleet unit: %v", err)
	}

	units, err := waitForNActiveUnits(1)
	if err != nil {
		t.Fatal(err)
	}
	_, found := units["hello.service"]
	if len(units) != 1 || !found {
		t.Fatalf("Expected hello.service to be sole active unit, got %v", units)
	}

	if _, _, err := fleetctl("stop", "hello.service"); err != nil {
		t.Fatal(err)
	}
	units, err = waitForNActiveUnits(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 0 {
		t.Fatal("Zero units should be running, found %v", units)
	}

	if _, _, err := fleetctl("start", "hello.service"); err != nil {
		t.Fatalf("Unable to start fleet unit: %v", err)
	}
	units, err = waitForNActiveUnits(1)
	if err != nil {
		t.Fatal(err)
	}
	_, found = units["hello.service"]
	if len(units) != 1 || !found {
		t.Fatalf("Expected hello.service to be sole active unit, got %v", units)
	}

}

func TestUnitSSHActions(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.DestroyAll()

	if err := cluster.CreateMultiple(1, platform.MachineConfig{}); err != nil {
		t.Fatal(err)
	}
	_, err = waitForNMachines(1)
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := fleetctl("start", "--no-block", "fixtures/units/hello.service"); err != nil {
		t.Fatalf("Unable to start fleet unit: %v", err)
	}

	units, err := waitForNActiveUnits(1)
	if err != nil {
		t.Fatal(err)
	}

	_, found := units["hello.service"]
	if len(units) != 1 || !found {
		t.Fatalf("Expected hello.service to be sole active unit, got %v", units)
	}

	stdout, _, err := fleetctl("--strict-host-key-checking=false", "ssh", "hello.service", "echo", "foo")
	if err != nil {
		t.Errorf("Failure occurred while calling fleetctl ssh: %v", err)
	}

	if !strings.Contains(stdout, "foo") {
		t.Errorf("Could not find expected string in command output:\n%s", stdout)
	}

	stdout, _, err = fleetctl("--strict-host-key-checking=false", "status", "hello.service")
	if err != nil {
		t.Errorf("Failure occurred while calling fleetctl status: %v", err)
	}

	if !strings.Contains(stdout, "Active: active") {
		t.Errorf("Could not find expected string in status output:\n%s", stdout)
	}

	stdout, _, err = fleetctl("--strict-host-key-checking=false", "journal", "hello.service")
	if err != nil {
		t.Errorf("Failure occurred while calling fleetctl journal: %v", err)
	}

	if !strings.Contains(stdout, "Hello, World!") {
		t.Errorf("Could not find expected string in journal output:\n%s", stdout)
	}
}
