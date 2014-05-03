package functional

import (
	"strings"
	"testing"

	"github.com/coreos/fleet/functional/platform"
)

// TestUnitRunnable is the simplest test possible, deplying a single-node
// cluster and ensuring a unit can enter an 'active' state
func TestUnitRunnable(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.Destroy()

	if err := platform.CreateNClusterMembers(cluster, 1, platform.MachineConfig{}); err != nil {
		t.Fatal(err)
	}
	_, err = cluster.WaitForNMachines(1)
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := cluster.Fleetctl("start", "fixtures/units/hello.service"); err != nil {
		t.Fatalf("Unable to start fleet unit: %v", err)
	}
	stdout, _, err := cluster.Fleetctl("list-units", "--no-legend")
	if err != nil {
		t.Fatalf("Failed to run list-units: %v", err)
	}
	units := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(units) != 1 {
		t.Fatalf("Did not find 1 unit in cluster: \n%s", stdout)
	}
	cols := strings.Split(units[0], "\t")
	if cols[3] != "active" {
		t.Fatalf("Unit did not enter active state")
	}
}

func TestUnitSubmit(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.Destroy()

	if err := platform.CreateNClusterMembers(cluster, 1, platform.MachineConfig{}); err != nil {
		t.Fatal(err)
	}
	_, err = cluster.WaitForNMachines(1)
	if err != nil {
		t.Fatal(err)
	}

	// submit a unit and assert it shows up
	if _, _, err := cluster.Fleetctl("submit", "fixtures/units/hello.service"); err != nil {
		t.Fatalf("Unable to submit fleet unit: %v", err)
	}
	stdout, _, err := cluster.Fleetctl("list-units", "--no-legend")
	if err != nil {
		t.Fatalf("Failed to run list-units: %v", err)
	}
	units := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(units) != 1 {
		t.Fatalf("Did not find 1 unit in cluster: \n%s", stdout)
	}

	// submitting the same unit should fail
	if _, _, err = cluster.Fleetctl("submit", "fixtures/units/hello.service"); err == nil {
		t.Fatalf("Expected failure when double-submitting unit, got success.")
	}

	// destroy the unit and ensure it disappears from the unit list
	if _, _, err := cluster.Fleetctl("destroy", "fixtures/units/hello.service"); err != nil {
		t.Fatalf("Failed to destroy unit: %v", err)
	}
	stdout, _, err = cluster.Fleetctl("list-units", "--no-legend")
	if err != nil {
		t.Fatalf("Failed to run list-units: %v", err)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("Did not find 0 units in cluster: \n%s", stdout)
	}

	// submitting the unit after destruction should succeed
	if _, _, err := cluster.Fleetctl("submit", "fixtures/units/hello.service"); err != nil {
		t.Fatalf("Unable to submit fleet unit: %v", err)
	}
	stdout, _, err = cluster.Fleetctl("list-units", "--no-legend")
	if err != nil {
		t.Fatalf("Failed to run list-units: %v", err)
	}
	units = strings.Split(strings.TrimSpace(stdout), "\n")
	if len(units) != 1 {
		t.Fatalf("Did not find 1 unit in cluster: \n%s", stdout)
	}
}

func TestUnitRestart(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.Destroy()

	if err := platform.CreateNClusterMembers(cluster, 1, platform.MachineConfig{}); err != nil {
		t.Fatal(err)
	}
	_, err = cluster.WaitForNMachines(1)
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := cluster.Fleetctl("start", "fixtures/units/hello.service"); err != nil {
		t.Fatalf("Unable to start fleet unit: %v", err)
	}

	units, err := cluster.WaitForNActiveUnits(1)
	if err != nil {
		t.Fatal(err)
	}
	_, found := units["hello.service"]
	if len(units) != 1 || !found {
		t.Fatalf("Expected hello.service to be sole active unit, got %v", units)
	}

	if _, _, err := cluster.Fleetctl("stop", "hello.service"); err != nil {
		t.Fatal(err)
	}
	units, err = cluster.WaitForNActiveUnits(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 0 {
		t.Fatalf("Zero units should be running, found %v", units)
	}

	if _, _, err := cluster.Fleetctl("start", "hello.service"); err != nil {
		t.Fatalf("Unable to start fleet unit: %v", err)
	}
	units, err = cluster.WaitForNActiveUnits(1)
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
	defer cluster.Destroy()

	if err := platform.CreateNClusterMembers(cluster, 1, platform.MachineConfig{}); err != nil {
		t.Fatal(err)
	}
	_, err = cluster.WaitForNMachines(1)
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := cluster.Fleetctl("start", "--no-block", "fixtures/units/hello.service"); err != nil {
		t.Fatalf("Unable to start fleet unit: %v", err)
	}

	units, err := cluster.WaitForNActiveUnits(1)
	if err != nil {
		t.Fatal(err)
	}

	_, found := units["hello.service"]
	if len(units) != 1 || !found {
		t.Fatalf("Expected hello.service to be sole active unit, got %v", units)
	}

	stdout, _, err := cluster.Fleetctl("--strict-host-key-checking=false", "ssh", "hello.service", "echo", "foo")
	if err != nil {
		t.Errorf("Failure occurred while calling fleetctl ssh: %v", err)
	}

	if !strings.Contains(stdout, "foo") {
		t.Errorf("Could not find expected string in command output:\n%s", stdout)
	}

	stdout, _, err = cluster.Fleetctl("--strict-host-key-checking=false", "status", "hello.service")
	if err != nil {
		t.Errorf("Failure occurred while calling fleetctl status: %v", err)
	}

	if !strings.Contains(stdout, "Active: active") {
		t.Errorf("Could not find expected string in status output:\n%s", stdout)
	}

	stdout, _, err = cluster.Fleetctl("--strict-host-key-checking=false", "journal", "hello.service")
	if err != nil {
		t.Errorf("Failure occurred while calling fleetctl journal: %v", err)
	}

	if !strings.Contains(stdout, "Hello, World!") {
		t.Errorf("Could not find expected string in journal output:\n%s", stdout)
	}
}
