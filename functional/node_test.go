/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

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
	if _, err := cluster.CreateMember("1", platform.MachineConfig{}); err != nil {
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
