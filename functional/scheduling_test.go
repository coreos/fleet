// Copyright 2014 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package functional

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coreos/fleet/functional/platform"
	"github.com/coreos/fleet/functional/util"
)

// Start three pairs of services, asserting each pair land on the same
// machine due to the MachineOf options in the unit files.
func TestScheduleMachineOf(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.Destroy(t)

	// Start with a simple three-node cluster
	members, err := platform.CreateNClusterMembers(cluster, 3)
	if err != nil {
		t.Fatal(err)
	}
	m0 := members[0]
	machines, err := cluster.WaitForNMachines(m0, 3)
	if err != nil {
		t.Fatal(err)
	}

	// Ensure we can SSH into each machine using fleetctl
	for _, machine := range machines {
		if stdout, stderr, err := cluster.Fleetctl(m0, "--strict-host-key-checking=false", "ssh", machine, "uptime"); err != nil {
			t.Errorf("Unable to SSH into fleet machine: \nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
		}
	}

	// Start the 3 pairs of services
	for i := 0; i < 3; i++ {
		ping := fmt.Sprintf("fixtures/units/ping.%d.service", i)
		pong := fmt.Sprintf("fixtures/units/pong.%d.service", i)
		stdout, stderr, err := cluster.Fleetctl(m0, "start", "--no-block", ping, pong)
		if err != nil {
			t.Errorf("Failed starting units: \nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
		}
	}

	// All 6 services should be visible immediately and become ACTIVE
	// shortly thereafter
	stdout, _, err := cluster.Fleetctl(m0, "list-unit-files", "--no-legend")
	if err != nil {
		t.Fatalf("Failed to run list-unit-files: %v", err)
	}
	units := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(units) != 6 {
		t.Fatalf("Did not find six units in cluster: \n%s", stdout)
	}
	active, err := cluster.WaitForNActiveUnits(m0, 6)
	if err != nil {
		t.Fatal(err)
	}
	states, err := util.ActiveToSingleStates(active)
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
	if _, _, err = cluster.Fleetctl(m0, "--strict-host-key-checking=false", "ssh", mach, "sudo", "systemctl", "stop", "fleet"); err != nil {
		t.Fatal(err)
	}

	var mN platform.Member
	if m0.ID() == states["ping.1.service"].Machine {
		mN = members[1]
	} else {
		mN = m0
	}

	if _, err := cluster.WaitForNMachines(mN, 2); err != nil {
		t.Fatal(err)
	}
	active, err = cluster.WaitForNActiveUnits(mN, 6)
	if err != nil {
		t.Fatal(err)
	}
	states, err = util.ActiveToSingleStates(active)
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
	defer cluster.Destroy(t)

	// Start with a simple three-node cluster
	members, err := platform.CreateNClusterMembers(cluster, 3)
	if err != nil {
		t.Fatal(err)
	}
	m0 := members[0]
	machines, err := cluster.WaitForNMachines(m0, 3)
	if err != nil {
		t.Fatal(err)
	}

	// Ensure we can SSH into each machine using fleetctl
	for _, machine := range machines {
		if stdout, stderr, err := cluster.Fleetctl(m0, "--strict-host-key-checking=false", "ssh", machine, "uptime"); err != nil {
			t.Errorf("Unable to SSH into fleet machine: \nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
		}
	}

	for i := 0; i < 5; i++ {
		unit := fmt.Sprintf("fixtures/units/conflict.%d.service", i)
		stdout, stderr, err := cluster.Fleetctl(m0, "start", "--no-block", unit)
		if err != nil {
			t.Errorf("Failed starting unit %s: \nstdout: %s\nstderr: %s\nerr: %v", unit, stdout, stderr, err)
		}
	}

	// All 5 services should be visible immediately and 3 should become
	// ACTIVE shortly thereafter
	stdout, _, err := cluster.Fleetctl(m0, "list-unit-files", "--no-legend")
	if err != nil {
		t.Fatalf("Failed to run list-unit-files: %v", err)
	}
	units := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(units) != 5 {
		t.Fatalf("Did not find five units in cluster: \n%s", stdout)
	}
	active, err := cluster.WaitForNActiveUnits(m0, 3)
	if err != nil {
		t.Fatal(err)
	}
	states, err := util.ActiveToSingleStates(active)
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

func TestScheduleOneWayConflict(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.Destroy(t)

	// Start with a simple three-node cluster
	members, err := platform.CreateNClusterMembers(cluster, 1)
	if err != nil {
		t.Fatal(err)
	}
	m0 := members[0]
	if _, err := cluster.WaitForNMachines(m0, 1); err != nil {
		t.Fatal(err)
	}

	// Start a unit that conflicts with a yet-to-be-scheduled unit
	name := "fixtures/units/conflicts-with-hello.service"
	if stdout, stderr, err := cluster.Fleetctl(m0, "start", "--no-block", name); err != nil {
		t.Fatalf("Failed starting unit %s: \nstdout: %s\nstderr: %s\nerr: %v", name, stdout, stderr, err)
	}

	active, err := cluster.WaitForNActiveUnits(m0, 1)
	if err != nil {
		t.Fatal(err)
	}
	states, err := util.ActiveToSingleStates(active)
	if err != nil {
		t.Fatal(err)
	}

	// Start a unit that has not defined conflicts
	name = "fixtures/units/hello.service"
	if stdout, stderr, err := cluster.Fleetctl(m0, "start", "--no-block", name); err != nil {
		t.Fatalf("Failed starting unit %s: \nstdout: %s\nstderr: %s\nerr: %v", name, stdout, stderr, err)
	}

	// Both units should show up, but only conflicts-with-hello.service
	// should report ACTIVE
	stdout, _, err := cluster.Fleetctl(m0, "list-unit-files", "--no-legend")
	if err != nil {
		t.Fatalf("Failed to run list-unit-files: %v", err)
	}
	units := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(units) != 2 {
		t.Fatalf("Did not find two units in cluster: \n%s", stdout)
	}
	active, err = cluster.WaitForNActiveUnits(m0, 1)
	if err != nil {
		t.Fatal(err)
	}
	states, err = util.ActiveToSingleStates(active)
	if err != nil {
		t.Fatal(err)
	}

	for unit := range states {
		if unit != "conflicts-with-hello.service" {
			t.Error("Incorrect unit started:", unit)
		}
	}

	// Destroying the conflicting unit should allow the other to start
	name = "conflicts-with-hello.service"
	if _, _, err := cluster.Fleetctl(m0, "destroy", name); err != nil {
		t.Fatalf("Failed destroying %s", name)
	}

	// Wait for the destroyed unit to actually disappear
	timeout, err := util.WaitForState(
		func() bool {
			stdout, _, err := cluster.Fleetctl(m0, "list-units", "--no-legend", "--full", "--fields", "unit,active,machine")
			if err != nil {
				return false
			}
			lines := strings.Split(strings.TrimSpace(stdout), "\n")
			states := util.ParseUnitStates(lines)
			for _, state := range states {
				if state.Name == name {
					return false
				}
			}
			return true
		},
	)
	if err != nil {
		t.Fatalf("Destroyed unit %s not gone within %v", name, timeout)
	}

	active, err = cluster.WaitForNActiveUnits(m0, 1)
	if err != nil {
		t.Fatal(err)
	}
	states, err = util.ActiveToSingleStates(active)
	if err != nil {
		t.Fatal(err)
	}
	for unit := range states {
		if unit != "hello.service" {
			t.Error("Incorrect unit started:", unit)
		}
	}

}

// TestScheduleReplace starts 1 unit, followed by starting another unit
// that replaces the 1st unit. Then it verifies that the 2 units are
// started on different machines.
func TestScheduleReplace(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.Destroy(t)

	// Start with a simple three-node cluster
	members, err := platform.CreateNClusterMembers(cluster, 2)
	if err != nil {
		t.Fatal(err)
	}
	m0 := members[0]
	if _, err := cluster.WaitForNMachines(m0, 2); err != nil {
		t.Fatal(err)
	}

	// Start a unit without Replaces
	uNames := []string{
		"fixtures/units/replace.0.service",
		"fixtures/units/replace.1.service",
	}
	if stdout, stderr, err := cluster.Fleetctl(m0, "start", "--no-block", uNames[0]); err != nil {
		t.Fatalf("Failed starting unit %s: \nstdout: %s\nstderr: %s\nerr: %v", uNames[0], stdout, stderr, err)
	}

	active, err := cluster.WaitForNActiveUnits(m0, 1)
	if err != nil {
		t.Fatal(err)
	}
	_, err = util.ActiveToSingleStates(active)
	if err != nil {
		t.Fatal(err)
	}

	// Start a unit that replaces the former one, replace.0.service
	if stdout, stderr, err := cluster.Fleetctl(m0, "start", "--no-block", uNames[1]); err != nil {
		t.Fatalf("Failed starting unit %s: \nstdout: %s\nstderr: %s\nerr: %v", uNames[1], stdout, stderr, err)
	}

	// Check that both units should show up
	stdout, _, err := cluster.Fleetctl(m0, "list-unit-files", "--no-legend")
	if err != nil {
		t.Fatalf("Failed to run list-unit-files: %v", err)
	}
	units := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(units) != 2 {
		t.Fatalf("Did not find two units in cluster: \n%s", stdout)
	}
	active, err = cluster.WaitForNActiveUnits(m0, 2)
	if err != nil {
		t.Fatal(err)
	}
	states, err := util.ActiveToSingleStates(active)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the unit 1 is located on a different machine from that of unit 0
	nUnits := 2
	uNameBase := make([]string, nUnits)
	machs := make([]string, nUnits)
	for i, uName := range uNames {
		uNameBase[i] = path.Base(uName)
		machs[i] = states[uNameBase[i]].Machine
	}
	if machs[0] == machs[1] {
		t.Fatalf("machine for %s is %s, the same as that of %s.", uNameBase[0], machs[0], uNameBase[1])
	}

	// Check that circular replaces end up with 1 launched unit.
	// First of all, stop the existing unit replace.0.service.
	if stdout, stderr, err := cluster.Fleetctl(m0, "destroy", uNameBase[0]); err != nil {
		t.Fatalf("Failed to destroy unit %s: \nstdout: %s\nstderr: %s\nerr: %v",
			uNameBase[0], stdout, stderr, err)
	}

	// Generate a new service 0 derived by a fixture, make the new service
	// replace service 1, and store it under /tmp.
	uName0tmp := path.Join("/tmp", uNameBase[0])
	err = util.GenNewFleetService(uName0tmp, uNames[1],
		"Replaces=replace.1.service", "Replaces=replace.0.service")
	if err != nil {
		t.Fatalf("Failed to generate a temp fleet service: %v", err)
	}

	// Start replace.0 unit that replaces replace.1.service,
	// then fleetctl list-unit-files should show only return 1 launched unit.
	// Note that we still need to run list-units once, before doing
	// list-unit-files, for reliable tests.
	stdout, stderr, err := cluster.Fleetctl(m0, "start", "--no-block", uName0tmp)
	if err != nil {
		t.Fatalf("Failed starting unit %s: \nstdout: %s\nstderr: %s\nerr: %v",
			uName0tmp, stdout, stderr, err)
	}

	stdout, _, err = cluster.Fleetctl(m0, "list-unit-files", "--no-legend")
	if err != nil {
		t.Fatalf("Failed to run list-unit-files: %v", err)
	}
	units = strings.Split(strings.TrimSpace(stdout), "\n")
	if len(units) != nUnits {
		t.Fatalf("Did not find two units in cluster: \n%s", stdout)
	}
	_, err = cluster.WaitForNActiveUnits(m0, nUnits)
	if err != nil {
		t.Fatal(err)
	}
	ufs, err := cluster.WaitForNUnitFiles(m0, nUnits)
	if err != nil {
		t.Fatalf("Failed to run list-unit-files: %v", err)
	}

	uStates := make([][]util.UnitFileState, nUnits)
	var found bool
	for i, unb := range uNameBase {
		uStates[i], found = ufs[unb]
		if len(ufs) != nUnits || !found {
			t.Fatalf("Did not find %d launched unit as expected: got %d\n", nUnits, len(ufs))
		}
	}
	nLaunched := 0
	for _, us := range uStates {
		for _, state := range us {
			if strings.Contains(state.State, "launched") {
				nLaunched += 1
			}
		}
	}
	if nLaunched != 1 {
		t.Fatalf("Did not find 1 launched unit as expected: got %d", nLaunched)
	}

	os.Remove(uName0tmp)
}

// Ensure units can be scheduled directly to a given machine using the
// MachineID unit option.
func TestScheduleConditionMachineID(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.Destroy(t)

	// Start with a simple three-node cluster
	members, err := platform.CreateNClusterMembers(cluster, 3)
	if err != nil {
		t.Fatal(err)
	}
	m0 := members[0]
	machines, err := cluster.WaitForNMachines(m0, 3)
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
MachineID=%s
`
		unitFile, err := util.TempUnit(fmt.Sprintf(contents, machine))
		if err != nil {
			t.Fatalf("Failed creating temporary unit: %v", err)
		}
		defer os.Remove(unitFile)

		stdout, stderr, err := cluster.Fleetctl(m0, "start", unitFile)
		if err != nil {
			t.Fatalf("Failed starting unit file %s: \nstdout: %s\nstderr: %s\nerr: %v", unitFile, stdout, stderr, err)
		}

		unit := filepath.Base(unitFile)
		schedule[unit] = machine
	}

	// Block until our three units have been started
	active, err := cluster.WaitForNActiveUnits(m0, 3)
	if err != nil {
		t.Fatal(err)
	}
	states, err := util.ActiveToSingleStates(active)
	if err != nil {
		t.Fatal(err)
	}

	for unit, unitState := range states {
		if unitState.Machine != schedule[unit] {
			t.Errorf("Unit %s was scheduled to %s, expected %s", unit, unitState.Machine, schedule[unit])
		}
	}
}

func TestScheduleGlobalUnits(t *testing.T) {
	// Create a three-member cluster
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.Destroy(t)
	members, err := platform.CreateNClusterMembers(cluster, 3)
	if err != nil {
		t.Fatal(err)
	}
	m0 := members[0]
	machines, err := cluster.WaitForNMachines(m0, 3)
	if err != nil {
		t.Fatal(err)
	}

	// Launch a couple of simple units
	stdout, stderr, err := cluster.Fleetctl(m0, "start", "--no-block", "fixtures/units/hello.service", "fixtures/units/goodbye.service")
	if err != nil {
		t.Fatalf("Failed starting units: \nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}

	// Both units should show up active
	_, err = cluster.WaitForNActiveUnits(m0, 2)
	if err != nil {
		t.Fatal(err)
	}

	// Now add a global unit
	stdout, stderr, err = cluster.Fleetctl(m0, "start", "--no-block", "fixtures/units/global.service")
	if err != nil {
		t.Fatalf("Failed starting unit: \nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}

	// Should see 2 + 3 units
	states, err := cluster.WaitForNActiveUnits(m0, 5)
	if err != nil {
		t.Fatal(err)
	}

	// Each machine should have a single global unit
	us := states["global.service"]
	for _, mach := range machines {
		var found bool
		for _, state := range us {
			if state.Machine == mach {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Did not find global unit on machine %v", mach)
			t.Logf("Found unit states:")
			for _, state := range states {
				t.Logf("%#v", state)
			}
		}
	}
}
