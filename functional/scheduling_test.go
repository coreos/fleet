// Copyright 2014 The fleet Authors
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

// +build all !dummytest

package functional

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	stdout, stderr, err := cluster.Fleetctl(m0, "list-unit-files", "--no-legend")
	if err != nil {
		t.Fatalf("Failed to run list-unit-files:\nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
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
	if stdout, stderr, err = cluster.Fleetctl(m0, "--strict-host-key-checking=false", "ssh", mach, "sudo", "systemctl", "stop", "fleet"); err != nil {
		t.Fatalf("Failed to stop fleet service:\nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
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
	stdout, stderr, err := cluster.Fleetctl(m0, "list-unit-files", "--no-legend")
	if err != nil {
		t.Fatalf("Failed to run list-unit-files:\nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
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
	stdout, stderr, err := cluster.Fleetctl(m0, "list-unit-files", "--no-legend")
	if err != nil {
		t.Fatalf("Failed to run list-unit-files:\nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
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
	if stdout, stderr, err := cluster.Fleetctl(m0, "destroy", name); err != nil {
		t.Fatalf("Failed destroying %s:\nstdout: %s\nstderr: %s\nerr: %v", name, stdout, stderr, err)
	}

	// NOTE: we need to sleep here shortly to avoid occasional errors of
	// conflicts-with-hello.service being rescheduled even after being destroyed.
	// In that case, the conflicts unit remains active, while the original
	// hello.service remains inactive. Then the test TestScheduleOneWayConflict
	// fails at the end with a message "Incorrect unit started".
	// This error seems to occur frequently when enable_grpc turned on.
	// - dpark 20160615
	time.Sleep(1 * time.Second)

	// Wait for the destroyed unit to actually disappear
	var stdoutBuf, stderrBuf string
	timeout, err := util.WaitForState(
		func() bool {
			stdoutBuf, stderrBuf, err = cluster.Fleetctl(m0, "list-units", "--no-legend", "--full", "--fields", "unit,active,machine")
			if err != nil {
				return false
			}
			lines := strings.Split(strings.TrimSpace(stdoutBuf), "\n")
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
		t.Fatalf("Destroyed unit %s not gone within %v\nstdout: %s\nstderr: %s\nerr: %v",
			name, timeout, stdoutBuf, stderrBuf, err)
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

// Start 3 services that conflict with one another.
func TestScheduleMultipleConflicts(t *testing.T) {
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

	for i := 0; i < 3; i++ {
		unit := fmt.Sprintf("fixtures/units/conflict.multiple.%d.service", i)
		stdout, stderr, err := cluster.Fleetctl(m0, "start", "--no-block", unit)
		if err != nil {
			t.Errorf("Failed starting unit %s: \nstdout: %s\nstderr: %s\nerr: %v", unit, stdout, stderr, err)
		}
	}

	// All 3 services should be visible immediately and 3 should become
	// ACTIVE shortly thereafter
	stdout, stderr, err := cluster.Fleetctl(m0, "list-unit-files", "--no-legend")
	if err != nil {
		t.Fatalf("Failed to run list-unit-files:\nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}
	units := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(units) != 3 {
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

// TestScheduleReplace starts 3 units, followed by starting another unit
// that replaces the 1st unit. Then it verifies that the original unit
// got rescheduled on a different machine.
func TestScheduleReplace(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.Destroy(t)

	members, err := platform.CreateNClusterMembers(cluster, 2)
	if err != nil {
		t.Fatal(err)
	}
	m0 := members[0]
	m1 := members[1]
	if _, err := cluster.WaitForNMachines(m0, 2); err != nil {
		t.Fatal(err)
	}

	// Start 3 units without Replaces, replace.0.service on m0, while both 1 and 2 on m1.
	// That's possible as replace.2.service has an option "MachineOf=replace.1.service".
	uNames := []string{
		"fixtures/units/replace.0.service",
		"fixtures/units/replace.1.service",
		"fixtures/units/replace.2.service",
		"fixtures/units/replace-kick0.service",
	}
	if stdout, stderr, err := cluster.Fleetctl(m0, "start", "--no-block", uNames[0]); err != nil {
		t.Fatalf("Failed starting unit %s: \nstdout: %s\nstderr: %s\nerr: %v", uNames[0], stdout, stderr, err)
	}
	if stdout, stderr, err := cluster.Fleetctl(m1, "start", "--no-block", uNames[1]); err != nil {
		t.Fatalf("Failed starting unit %s: \nstdout: %s\nstderr: %s\nerr: %v", uNames[1], stdout, stderr, err)
	}
	if stdout, stderr, err := cluster.Fleetctl(m1, "start", "--no-block", uNames[2]); err != nil {
		t.Fatalf("Failed starting unit %s: \nstdout: %s\nstderr: %s\nerr: %v", uNames[2], stdout, stderr, err)
	}

	active, err := cluster.WaitForNActiveUnits(m0, 3)
	if err != nil {
		t.Fatal(err)
	}
	states, err := util.ActiveToSingleStates(active)
	if err != nil {
		t.Fatal(err)
	}

	oldMach := states[path.Base(uNames[0])].Machine

	// Start a unit replace-kick0.service that replaces replace.0.service
	// Then the kick0 unit will be scheduled to m0, as m0 is least loaded than m1.
	// So it's possible to trigger a situation where kick0 could kick the original unit 0.
	if stdout, stderr, err := cluster.Fleetctl(m0, "start", "--no-block", uNames[3]); err != nil {
		t.Fatalf("Failed starting unit %s: \nstdout: %s\nstderr: %s\nerr: %v", uNames[3], stdout, stderr, err)
	}

	// Here we need to wait up to 15 seconds, to avoid races, because the unit state
	// publisher could otherwise report unit states with old machine IDs to registry.
	checkReplacedMachines := func() bool {
		// Check that 4 units show up
		nUnits := 4
		stdout, stderr, err := cluster.Fleetctl(m0, "list-unit-files", "--no-legend")
		if err != nil {
			t.Logf("Failed to run list-unit-files:\nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
			return false
		}
		units := strings.Split(strings.TrimSpace(stdout), "\n")
		if len(units) != nUnits {
			t.Logf("Did not find two units in cluster: \n%s", stdout)
			return false
		}
		active, err = cluster.WaitForNActiveUnits(m0, nUnits)
		if err != nil {
			t.Log(err)
			return false
		}
		states, err = util.ActiveToSingleStates(active)
		if err != nil {
			t.Log(err)
			return false
		}

		// Check that replace.0.service is located on a different machine from
		// that of replace-kick0.service.
		uNameBase := make([]string, nUnits)
		machs := make([]string, nUnits)
		for i, uName := range uNames {
			uNameBase[i] = path.Base(uName)
			machs[i] = states[uNameBase[i]].Machine
		}
		if machs[0] == machs[3] {
			t.Logf("machine for %s is %s, the same as that of %s.", uNameBase[0], machs[0], uNameBase[3])
			return false
		}
		if machs[3] != oldMach {
			t.Logf("machine for %s is %s, different from old machine %s.", uNameBase[3], machs[3], oldMach)
			return false
		}
		if machs[0] == oldMach {
			t.Logf("machine for %s is %s, the same as that of %s.", uNameBase[0], machs[0], oldMach)
			return false
		}

		return true
	}
	if timeout, err := util.WaitForState(checkReplacedMachines); err != nil {
		t.Fatalf("Cannot verify replaced units within %v\nerr: %v", timeout, err)
	}
}

// TestScheduleCircularReplace starts 2 units that tries to replace each other.
// Thus it's expected that only one of the units becomes active.
func TestScheduleCircularReplace(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.Destroy(t)

	members, err := platform.CreateNClusterMembers(cluster, 2)
	if err != nil {
		t.Fatal(err)
	}
	m0 := members[0]
	if _, err := cluster.WaitForNMachines(m0, 2); err != nil {
		t.Fatal(err)
	}

	// Check that circular replaces end up with 1 launched unit.
	// To do that, generate a new service 0 that replaces service 1, and store
	// it under /tmp. Also store the original service 1 that replace 0.
	uNames := []string{
		"fixtures/units/replace.0.service",
		"fixtures/units/replace-kick0.service",
	}
	nUnits := 2
	nActiveUnits := 1
	uNameBase := make([]string, nUnits)
	for i, uName := range uNames {
		uNameBase[i] = path.Base(uName)
	}
	uName0tmp := path.Join("/tmp", uNameBase[0])
	err = util.GenNewFleetService(uName0tmp, uNames[1],
		"Replaces=replace-kick0.service", "Replaces=replace.0.service")
	if err != nil {
		t.Fatalf("Failed to generate a temp fleet service: %v", err)
	}

	// Start replace.0 unit that replaces replace-kick0.service,
	// then fleetctl list-unit-files should show only return 1 launched unit.
	stdout, stderr, err := cluster.Fleetctl(m0, "start", "--no-block", uName0tmp)
	if err != nil {
		t.Fatalf("Failed starting unit %s: \nstdout: %s\nstderr: %s\nerr: %v",
			uName0tmp, stdout, stderr, err)
	}

	stdout, stderr, err = cluster.Fleetctl(m0, "list-unit-files", "--no-legend")
	if err != nil {
		t.Fatalf("Failed to run list-unit-files:\nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}
	units := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(units) != nActiveUnits {
		t.Fatalf("Did not find two units in cluster: \n%s", stdout)
	}
	_, err = cluster.WaitForNActiveUnits(m0, nActiveUnits)
	if err != nil {
		t.Fatal(err)
	}
	ufs, err := cluster.WaitForNUnitFiles(m0, nActiveUnits)
	if err != nil {
		t.Fatalf("Failed to run list-unit-files: %v", err)
	}

	// Start replace-kick0 unit that replaces replace.0.service,
	// and then check that only 1 unit is active
	if stdout, stderr, err := cluster.Fleetctl(m0, "start", "--no-block", uNames[1]); err != nil {
		t.Fatalf("Failed starting unit %s: \nstdout: %s\nstderr: %s\nerr: %v", uNames[1], stdout, stderr, err)
	}
	stdout, stderr, err = cluster.Fleetctl(m0, "list-unit-files", "--no-legend")
	if err != nil {
		t.Fatalf("Failed to run list-unit-files:\nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}
	units = strings.Split(strings.TrimSpace(stdout), "\n")
	if len(units) != nUnits {
		t.Fatalf("Did not find %d units in cluster: \n%s", nUnits, stdout)
	}

	active, err := cluster.WaitForNActiveUnits(m0, nActiveUnits)
	if err != nil {
		t.Fatal(err)
	}
	_, err = util.ActiveToSingleStates(active)
	if err != nil {
		t.Fatal(err)
	}

	uStates := make([][]util.UnitFileState, nUnits)
	for i, unb := range uNameBase {
		uStates[i], _ = ufs[unb]
	}
	nLaunched := 0
	for _, us := range uStates {
		for _, state := range us {
			if strings.Contains(state.State, "launched") {
				nLaunched += 1
			}
		}
	}
	if nLaunched != nActiveUnits {
		t.Fatalf("Did not find %d launched unit as expected: got %d", nActiveUnits, nLaunched)
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
	numGUnits := 3
	numAllUnits := 5
	members, err := platform.CreateNClusterMembers(cluster, numGUnits)
	if err != nil {
		t.Fatal(err)
	}
	m0 := members[0]
	machines, err := cluster.WaitForNMachines(m0, numGUnits)
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
	globalLongPath := "fixtures/units/global.service"
	stdout, stderr, err = cluster.Fleetctl(m0, "start", "--no-block", globalLongPath)
	if err != nil {
		t.Fatalf("Failed starting unit: \nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}

	// Should see 2 + 3 units
	states, err := cluster.WaitForNActiveUnits(m0, numAllUnits)
	if err != nil {
		t.Fatal(err)
	}

	// Each machine should have a single global unit
	globalBase := path.Base(globalLongPath)
	us := states[globalBase]
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

	stdout, stderr, err = cluster.Fleetctl(m0, "status", "--no-block", globalBase)
	if err != nil {
		t.Fatalf("Failed getting unit status: \nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}

	// restarting global units
	cmd := "restart"
	stdout, stderr, err = cluster.Fleetctl(m0, cmd, globalBase)
	if err != nil {
		t.Fatalf("Failed restarting global unit: \nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}

	outmap, err := waitForNUnitsStart(cluster, m0, numAllUnits)
	if err != nil {
		t.Fatalf("Failed listing global units: %v", err)
	}
	glist, _ := outmap[globalBase]
	if len(glist) != numGUnits {
		t.Fatalf("Did not find %d global units: got %d", numGUnits, len(glist))
	}
}

// TestScheduleGlobalConflicts starts 2 global units that conflict with each
// other, and check if only the first one can be found.
func TestScheduleGlobalConflicts(t *testing.T) {
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

	cfGlobal0 := "fixtures/units/conflict-global.0.service"
	cfGlobal1 := "fixtures/units/conflict-global.1.service"

	// Launch a global unit
	stdout, stderr, err := cluster.Fleetctl(m0, "start", "--no-block", cfGlobal0)
	if err != nil {
		t.Fatalf("Failed starting units: \nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}

	// the global unit should show up active on 3 machines
	_, err = cluster.WaitForNActiveUnits(m0, 3)
	if err != nil {
		t.Fatal(err)
	}

	// Now add another global unit, which actually should not be started.
	stdout, stderr, err = cluster.Fleetctl(m0, "start", "--no-block", cfGlobal1)
	if err != nil {
		t.Fatalf("Failed starting unit: \nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}

	// Should see only 3 units
	states, err := cluster.WaitForNActiveUnits(m0, 3)
	if err != nil {
		t.Fatal(err)
	}

	// Each machine should have a single global unit conflict-global.0.service,
	// but not conflict-global.1.service.
	us0 := states[path.Base(cfGlobal0)]
	us1 := states[path.Base(cfGlobal1)]
	for _, mach := range machines {
		var found bool
		for _, state := range us0 {
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

		found = false
		for _, state := range us1 {
			if state.Machine == mach {
				found = true
				break
			}
		}
		if found {
			t.Fatalf("Did find global unit %s on machine %v", us1, mach)
			t.Logf("Global units were not conflicted as expected.")
		}
	}
}
