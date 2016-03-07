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

	// Start with a single node and wait for it to come up
	m0, err := cluster.CreateMember()
	if err != nil {
		t.Fatal(err)
	}
	machines, err := cluster.WaitForNMachines(m0, 1)
	if err != nil {
		t.Fatal(err)
	}

	// Start a unit and ensure it comes up quickly
	unit := fmt.Sprintf("fixtures/units/pin@%s.service", machines[0])
	stdout, stderr, err := cluster.Fleetctl(m0, "start", unit)
	if err != nil {
		t.Errorf("Failed starting unit: \nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}
	_, err = cluster.WaitForNActiveUnits(m0, 1)
	if err != nil {
		t.Fatal(err)
	}

	// Create a second node, waiting for it
	m1, err := cluster.CreateMember()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = cluster.WaitForNMachines(m0, 2); err != nil {
		t.Fatal(err)
	}

	// Stop the fleet process on the first member
	if _, err = cluster.MemberCommand(m0, "sudo", "systemctl", "stop", "fleet"); err != nil {
		t.Fatal(err)
	}

	// The first member should quickly remove itself from the published
	// list of cluster members
	if _, err = cluster.WaitForNMachines(m1, 1); err != nil {
		t.Fatal(err)
	}

	// State for the member's unit should be purged from the Registry
	if _, err = cluster.WaitForNActiveUnits(m1, 0); err != nil {
		t.Fatal(err)
	}

	// The member's unit should actually stop running, too
	stdout, _ = cluster.MemberCommand(m0, "sudo", "systemctl", "status", "hello.service")
	if !strings.Contains(stdout, "Active: inactive") {
		t.Fatalf("Unit hello.service not reported as inactive:\n%s\n", stdout)
	}
}
