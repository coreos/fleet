package functional

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coreos/fleet/functional/platform"
)

// Start three pairs of services, asserting each pair land on the same
// machine due to the X-ConditionMachineOf options in the unit files.
func TestScheduleConditionMachineOf(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.Destroy()

	// Start with a simple three-node cluster
	if err := platform.CreateNClusterMembers(cluster, 3, platform.MachineConfig{}); err != nil {
		t.Fatal(err)
	}
	machines, err := waitForNMachines(3)
	if err != nil {
		t.Fatal(err)
	}

	// Ensure we can SSH into each machine using fleetctl
	for _, machine := range machines {
		if _, _, err := fleetctl("--strict-host-key-checking=false", "ssh", machine, "uptime"); err != nil {
			t.Errorf("Unable to SSH into fleet machine: %v", err)
		}
	}

	// Start the 3 pairs of services
	for i := 0; i < 3; i++ {
		ping := fmt.Sprintf("fixtures/units/ping.%d.service", i)
		pong := fmt.Sprintf("fixtures/units/pong.%d.service", i)
		_, _, err := fleetctl("start", "--no-block", ping, pong)
		if err != nil {
			t.Errorf("Failed starting units: %v", err)
		}
	}

	// All 6 services should be visible immediately and become ACTIVE
	// shortly thereafter
	stdout, _, err := fleetctl("list-units", "--no-legend")
	if err != nil {
		t.Fatalf("Failed to run list-units: %v", err)
	}
	units := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(units) != 6 {
		t.Fatalf("Did not find six units in cluster: \n%s", stdout)
	}
	states, err := waitForNActiveUnits(6)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 3; i++ {
		ping := fmt.Sprintf("ping.%d.service", i)
		pingState, ok := states[ping]
		if !ok {
			t.Errorf("Failed to find state for %s", ping)
			continue
		}

		pong := fmt.Sprintf("pong.%d.service", i)
		pongState, ok := states[pong]
		if !ok {
			t.Errorf("Failed to find state for %s", pong)
			continue
		}

		if len(pingState.Machine) == 0 {
			t.Errorf("Unit %s is not reporting machine", ping)
		}

		if len(pongState.Machine) == 0 {
			t.Errorf("Unit %s is not reporting machine", pong)
		}

		if pingState.Machine != pongState.Machine {
			t.Errorf("Units %s and %s are not on same machine", ping, pong)
		}
	}

	// Ensure a pair of units migrate together when their host goes down
	mach := states["ping.1.service"].Machine
	if _, _, err = fleetctl("--strict-host-key-checking=false", "ssh", mach, "sudo", "systemctl", "stop", "fleet"); err != nil {
		t.Fatal(err)
	}
	if _, err := waitForNMachines(2); err != nil {
		t.Fatal(err)
	}
	states, err = waitForNActiveUnits(6)
	if err != nil {
		t.Fatal(err)
	}

	newPingMach := states["ping.1.service"].Machine
	if mach == newPingMach {
		t.Fatalf("Unit ping.1.service did not appear to migrate")
	}

	newPongMach := states["pong.1.service"].Machine
	if newPingMach != newPongMach {
		t.Errorf("Unit pong.1.service did not migrate with ping.1.service")
	}
}

// Start 5 services that conflict with one another. Assert that only
// 3 of the 5 are started.
func TestScheduleConflicts(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.Destroy()

	// Start with a simple three-node cluster
	if err := platform.CreateNClusterMembers(cluster, 3, platform.MachineConfig{}); err != nil {
		t.Fatal(err)
	}
	machines, err := waitForNMachines(3)
	if err != nil {
		t.Fatal(err)
	}

	// Ensure we can SSH into each machine using fleetctl
	for _, machine := range machines {
		if _, _, err := fleetctl("--strict-host-key-checking=false", "ssh", machine, "uptime"); err != nil {
			t.Errorf("Unable to SSH into fleet machine: %v", err)
		}
	}

	// submit 5 conflicting pairs of services
	for i := 0; i < 5; i++ {
		unit := fmt.Sprintf("fixtures/units/conflict.%d.service", i)
		_, _, err := fleetctl("submit", unit)
		if err != nil {
			t.Errorf("Failed submitting unit %s: %v", unit, err)
		}
	}

	// starting the first 3 should be fine
	for i := 0; i < 3; i++ {
		unit := fmt.Sprintf("fixtures/units/conflict.%d.service", i)
		_, _, err := fleetctl("start", unit)
		if err != nil {
			t.Errorf("Failed starting unit %s: %v", unit, err)
		}
	}

	// starting the last 2 should fail?
	for i := 3; i < 5; i++ {
		unit := fmt.Sprintf("fixtures/units/conflict.%d.service", i)
		_, _, err := fleetctl("start", unit)
		if err == nil {
			t.Errorf("Expected nonzero exit code while starting unscheduleable unit %s: %v", unit, err)
		}
	}

	// All 5 services should be visible immediately and 3 should become
	// ACTIVE shortly thereafter
	stdout, _, err := fleetctl("list-units", "--no-legend")
	if err != nil {
		t.Fatalf("Failed to run list-units: %v", err)
	}
	units := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(units) != 5 {
		t.Fatalf("Did not find five units in cluster: \n%s", stdout)
	}
	states, err := waitForNActiveUnits(3)
	if err != nil {
		t.Fatal(err)
	}

	machineSet := make(map[string]bool)

	for unit, unitState := range states {
		if len(unitState.Machine) == 0 {
			t.Errorf("Unit %s is not reporting machine", unit)
		}

		machineSet[unitState.Machine] = true
	}

	if len(machineSet) != 3 {
		t.Errorf("3 active units not running on 3 unique machines")
	}
}

// Ensure units can be scheduled directly to a given machine using the
// X-ConditionMachineBootID unit option.
func TestScheduleConditionMachineBootID(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.Destroy()

	// Start with a simple three-node cluster
	if err := platform.CreateNClusterMembers(cluster, 3, platform.MachineConfig{}); err != nil {
		t.Fatal(err)
	}
	machines, err := waitForNMachines(3)
	if err != nil {
		t.Fatal(err)
	}

	// Start 3 units that are each scheduled to one of our machines
	schedule := make(map[string]string)
	for _, machine := range machines {
		contents := `
[Service]
ExecStart=/bin/bash -c "while true; do echo Hello, World!; sleep 1; done"

[X-Fleet]
X-ConditionMachineBootID=%s
`
		unitFile, err := tempUnit(fmt.Sprintf(contents, machine))
		if err != nil {
			t.Fatalf("Failed creating temporary unit: %v", err)
		}
		defer os.Remove(unitFile)

		_, _, err = fleetctl("start", unitFile)
		if err != nil {
			t.Fatalf("Failed starting unit file %s: %v", unitFile, err)
		}

		unit := filepath.Base(unitFile)
		schedule[unit] = machine
	}

	// Block until our three units have been started
	states, err := waitForNActiveUnits(3)
	if err != nil {
		t.Fatal(err)
	}

	for unit, unitState := range states {
		if unitState.Machine != schedule[unit] {
			t.Errorf("Unit %s was scheduled to %s, expected %s", unit, unitState.Machine, schedule[unit])
		}
	}
}
