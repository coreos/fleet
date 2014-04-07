package functional

import (
	"strings"
	"testing"

	"github.com/coreos/fleet/functional/platform"
)

// Ensure an existing unit migrates to an unoccupied machine
// if its host goes down.
func TestDynamicClusterNewMemberUnitMigration(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.Destroy()

	// Start with a 4-node cluster
	if err := platform.CreateNClusterMembers(cluster, 4, platform.MachineConfig{}); err != nil {
		t.Fatal(err)
	}
	if _, err = waitForNMachines(4); err != nil {
		t.Fatal(err)
	}

	// Start 3 conflicting units on the 4-node cluster
	if _, _, err := fleetctl("start",
		"fixtures/units/conflict.0.service",
		"fixtures/units/conflict.1.service",
		"fixtures/units/conflict.2.service",
	); err != nil {
		t.Errorf("Failed starting units: %v", err)
	}

	// All 3 services should be visible immediately, and all of them should
	// become ACTIVE shortly thereafter
	stdout, _, err := fleetctl("list-units", "--no-legend")
	if err != nil {
		t.Fatalf("Failed to run list-units: %v", err)
	}
	units := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(units) != 3 {
		t.Fatalf("Did not find 3 units in cluster: \n%s", stdout)
	}
	states, err := waitForNActiveUnits(3)
	if err != nil {
		t.Fatal(err)
	}

	// Kill one of the machines and make sure the unit migrates somewhere else
	unit := "conflict.1.service"
	oldMach := states[unit].Machine
	if _, _, err = fleetctl("--strict-host-key-checking=false", "ssh", oldMach, "sudo", "systemctl", "stop", "fleet"); err != nil {
		t.Fatal(err)
	}
	if _, err = waitForNMachines(3); err != nil {
		t.Fatal(err)
	}
	newStates, err := waitForNActiveUnits(3)
	if err != nil {
		t.Fatal(err)
	}
	newMach := newStates[unit].Machine
	if newMach == oldMach {
		t.Fatalf("Unit %s did not migrate from machine %s to %s", unit, oldMach, newMach)
	}

	// Ensure no other units migrated due to this churn
	if newMach == states["conflict.0.service"].Machine || newMach == states["conflict.2.service"].Machine {
		t.Errorf("Unit %s landed on occupied machine")
	}

	if states["conflict.0.service"].Machine != newStates["conflict.0.service"].Machine || states["conflict.2.service"].Machine != newStates["conflict.2.service"].Machine {
		t.Errorf("Unit caused unnecessary churn in the cluster")
	}
}

// Simulate rebooting a single member of a fleet cluster
func TestDynamicClusterMemberReboot(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.Destroy()

	// Start with a simple three-node cluster
	if err := platform.CreateNClusterMembers(cluster, 3, platform.MachineConfig{}); err != nil {
		t.Fatal(err)
	}
	if _, err = waitForNMachines(3); err != nil {
		t.Fatal(err)
	}

	if _, _, err := fleetctl("start",
		"fixtures/units/conflict.0.service",
		"fixtures/units/conflict.1.service",
		"fixtures/units/conflict.2.service",
	); err != nil {
		t.Errorf("Failed starting units: %v", err)
	}

	// All 3 services should be visible immediately, and all of them should
	// become ACTIVE shortly thereafter
	stdout, _, err := fleetctl("list-units", "--no-legend")
	if err != nil {
		t.Fatalf("Failed to run list-units: %v", err)
	}
	units := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(units) != 3 {
		t.Fatalf("Did not find 3 units in cluster: \n%s", stdout)
	}
	oldStates, err := waitForNActiveUnits(3)
	if err != nil {
		t.Fatal(err)
	}

	// Simulate a reboot by recreating one of the cluster members
	member := cluster.Members()[1]
	if _, err = cluster.MemberCommand(member, "sudo", "systemctl", "stop", "fleet"); err != nil {
		t.Fatal(err)
	}
	if err = cluster.DestroyMember(member); err != nil {
		t.Fatal(err)
	}
	if _, err = waitForNMachines(2); err != nil {
		t.Fatal(err)
	}
	if _, err = waitForNActiveUnits(2); err != nil {
		t.Fatal(err)
	}
	if err = cluster.CreateMember(member, platform.MachineConfig{}); err != nil {
		t.Fatal(err)
	}
	if _, err = waitForNMachines(3); err != nil {
		t.Fatal(err)
	}
	newStates, err := waitForNActiveUnits(3)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, unit := range []string{"conflict.0.service", "conflict.1.service", "conflict.2.service"} {
		if oldStates[unit].Machine != newStates[unit].Machine {
			if found {
				t.Fatal("More than one unit migrated")
			} else {
				found = true
			}
		}
	}

	if !found {
		t.Fatal("Could not find migrated unit")
	}
}
