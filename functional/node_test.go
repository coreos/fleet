package functional

import (
	"strings"
	"testing"

	"github.com/coreos/fleet/functional/platform"
)

// Simulate the shutdown of a single fleet node
func TestNodeShutdown(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.Destroy()

	// Start with a single-node cluster
	if err := cluster.CreateMember("1", platform.MachineConfig{}); err != nil {
		t.Fatal(err)
	}
	if _, err = cluster.WaitForNMachines(1); err != nil {
		t.Fatal(err)
	}

	// Start a unit and ensure it comes up quickly
	if _, _, err := cluster.Fleetctl("start", "fixtures/units/hello.service"); err != nil {
		t.Errorf("Failed starting unit: %v", err)
	}
	_, err = cluster.WaitForNActiveUnits(1)
	if err != nil {
		t.Fatal(err)
	}

	// Stop the fleet process on our sole member
	if _, err = cluster.MemberCommand("1", "sudo", "systemctl", "stop", "fleet"); err != nil {
		t.Fatal(err)
	}

	// The member should immediately remove itself from the published
	// list of cluster members
	if _, err = cluster.WaitForNMachines(0); err != nil {
		t.Fatal(err)
	}

	// State for the members units should be purged from the Registry
	if _, err = cluster.WaitForNActiveUnits(0); err != nil {
		t.Fatal(err)
	}

	// The members units should actually stop running, too
	stdout, _ := cluster.MemberCommand("1", "sudo", "systemctl", "status", "hello.service")
	if !strings.Contains(stdout, "Active: inactive") {
		t.Fatalf("Unit hello.service not reported as inactive:\n%s\n", stdout)
	}
}
