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
	"strings"
	"testing"

	"github.com/coreos/fleet/functional/platform"
	"github.com/coreos/fleet/functional/util"
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
	members, err := platform.CreateNClusterMembers(cluster, 4)
	if err != nil {
		t.Fatal(err)
	}
	m0 := members[0]
	if _, err = cluster.WaitForNMachines(m0, 4); err != nil {
		t.Fatal(err)
	}

	// Start 3 conflicting units on the 4-node cluster
	stdout, stderr, err := cluster.Fleetctl(m0, "start",
		"fixtures/units/conflict.0.service",
		"fixtures/units/conflict.1.service",
		"fixtures/units/conflict.2.service",
	)
	if err != nil {
		t.Errorf("Failed starting units: \nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}

	// All 3 services should be visible immediately, and all of them should
	// become ACTIVE shortly thereafter
	stdout, _, err = cluster.Fleetctl(m0, "list-units", "--no-legend")
	if err != nil {
		t.Fatalf("Failed to run list-units: %v", err)
	}
	units := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(units) != 3 {
		t.Fatalf("Did not find 3 units in cluster: \n%s", stdout)
	}
	active, err := cluster.WaitForNActiveUnits(m0, 3)
	if err != nil {
		t.Fatal(err)
	}

	// Ensure each unit is only running on a single machine
	states, err := util.ActiveToSingleStates(active)
	if err != nil {
		t.Fatal(err)
	}

	// Kill one of the machines and make sure the unit migrates somewhere else
	unit := "conflict.1.service"
	oldMach := states[unit].Machine
	if _, _, err = cluster.Fleetctl(m0, "--strict-host-key-checking=false", "ssh", oldMach, "sudo", "systemctl", "stop", "fleet"); err != nil {
		t.Fatal(err)
	}
	var mN platform.Member
	if m0.ID() == oldMach {
		mN = members[1]
	} else {
		mN = m0
	}

	if _, err = cluster.WaitForNMachines(mN, 3); err != nil {
		t.Fatal(err)
	}
	newActive, err := cluster.WaitForNActiveUnits(mN, 3)
	if err != nil {
		t.Fatal(err)
	}
	// Ensure each unit is only running on a single machine
	newStates, err := util.ActiveToSingleStates(newActive)
	if err != nil {
		t.Fatal(err)
	}

	newMach := newStates[unit].Machine
	if newMach == oldMach {
		t.Fatalf("Unit %s did not migrate from machine %s to %s", unit, oldMach, newMach)
	}

	// Ensure no other units migrated due to this churn
	if newMach == states["conflict.0.service"].Machine || newMach == states["conflict.2.service"].Machine {
		t.Errorf("Unit %s landed on occupied machine", unit)
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
	members, err := platform.CreateNClusterMembers(cluster, 3)
	if err != nil {
		t.Fatal(err)
	}
	m0 := members[0]
	if _, err = cluster.WaitForNMachines(m0, 3); err != nil {
		t.Fatal(err)
	}

	_, _, err = cluster.Fleetctl(m0, "start",
		"fixtures/units/conflict.0.service",
		"fixtures/units/conflict.1.service",
		"fixtures/units/conflict.2.service",
	)
	if err != nil {
		t.Errorf("Failed starting units: %v", err)
	}

	// All 3 services should be visible immediately, and all of them should
	// become ACTIVE shortly thereafter
	stdout, _, err := cluster.Fleetctl(m0, "list-units", "--no-legend")
	if err != nil {
		t.Fatalf("Failed to run list-units: %v", err)
	}
	units := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(units) != 3 {
		t.Fatalf("Did not find 3 units in cluster: \n%s", stdout)
	}
	oldActive, err := cluster.WaitForNActiveUnits(m0, 3)
	if err != nil {
		t.Fatal(err)
	}

	oldStates, err := util.ActiveToSingleStates(oldActive)
	if err != nil {
		t.Fatal(err)
	}

	// Simulate a reboot by recreating one of the cluster members
	if _, err := cluster.ReplaceMember(cluster.Members()[1]); err != nil {
		t.Fatalf("replace failed: %v", err)
	}
	newActive, err := cluster.WaitForNActiveUnits(m0, 3)
	if err != nil {
		t.Fatal(err)
	}
	newStates, err := util.ActiveToSingleStates(newActive)
	if err != nil {
		t.Fatal(err)
	}

	migrated := 0
	for _, unit := range []string{"conflict.0.service", "conflict.1.service", "conflict.2.service"} {
		if oldStates[unit].Machine != newStates[unit].Machine {
			migrated += 1
		}
	}

	if migrated != 1 {
		t.Errorf("Expected 1 unit to migrate, but found %d", migrated)
		t.Logf("Initial state: %#v", oldStates)
		t.Logf("Post-reboot state: %#v", newStates)
	}
}
