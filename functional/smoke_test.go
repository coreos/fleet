package functional

import (
	"fmt"
	"strings"
	"testing"

	"github.com/coreos/fleet/functional/platform"
)

func TestSmoke(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatalf(err.Error())
	}
	defer cluster.DestroyAll()

	// Start with a simple three-node cluster
	if err := cluster.CreateMultiple(3, platform.MachineConfig{}); err != nil {
		t.Fatalf(err.Error())
	}
	machines, err := waitForNMachines(3)
	if err != nil {
		t.Fatalf(err.Error())
	}

	// Ensure we can SSH into each machine using fleetctl
	for _, machine := range machines {
		if _, _, err := fleetctl("--strict-host-key-checking=false", "ssh", machine, "uptime"); err != nil {
			t.Errorf("Unable to SSH into fleet machine: %v", err)
		}
	}

	// Start the 5 services
	for i := 0; i < 5; i++ {
		unitName := fmt.Sprintf("fixtures/units/conflict.%d.service", i)
		_, _, err := fleetctl("start", "--no-block", unitName)
		if err != nil {
			t.Errorf("Failed starting %s: %v", unitName, err)
		}
	}

	// All 5 services should be visible immediately and become ACTIVE
	// shortly thereafter
	stdout, _, err := fleetctl("list-units", "--no-legend")
	if err != nil {
		t.Fatalf("Failed to run list-units: %v", err)
	}
	units := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(units) != 5 {
		t.Fatalf("Did not find five units in cluster: \n%s", stdout)
	}
	if _, err := waitForNActiveUnits(3); err != nil {
		t.Fatalf(err.Error())
	}

	// Add two more machines to the cluster and ensure the remaining
	// unscheduled services are picked up.
	if err := cluster.CreateMultiple(2, platform.MachineConfig{}); err != nil {
		t.Fatalf(err.Error())
	}
	machines, err = waitForNMachines(5)
	if err != nil {
		t.Fatalf(err.Error())
	}
	if _, err := waitForNActiveUnits(5); err != nil {
		t.Fatalf(err.Error())
	}
}
