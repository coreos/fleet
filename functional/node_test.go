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
	"github.com/coreos/fleet/functional/util"
)

// Simulate the shutdown of a single fleet node
func TestNodeShutdown(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.Destroy(t)

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
	stdout, _ = cluster.MemberCommand(m0, "systemctl", "status", "hello.service")
	if !strings.Contains(stdout, "Active: inactive") {
		t.Fatalf("Unit hello.service not reported as inactive:\n%s\n", stdout)
	}
}

// TestDetectMachineId checks for etcd registration failing on a duplicated
// machine-id on different machines.
// First it creates a cluster with 2 members, m0 and m1. Then make their
// machine IDs the same as each other, by explicitly setting the m1's ID to
// the same as m0's. Test succeeds when an error returns, while test fails
// when nothing happens.
func TestDetectMachineId(t *testing.T) {
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
	_, err = cluster.WaitForNMachines(m0, 2)
	if err != nil {
		t.Fatal(err)
	}

	machineIdFile := "/etc/machine-id"

	// Restart fleet service, and check if its systemd status is still active.
	restartFleetService := func(m platform.Member) error {
		stdout, err := cluster.MemberCommand(m, "sudo", "systemctl", "restart", "fleet.service")
		if err != nil {
			return fmt.Errorf("Failed to restart fleet service\nstdout: %s\nerr: %v", stdout, err)
		}

		stdout, _ = cluster.MemberCommand(m, "systemctl", "show", "--property=ActiveState", "fleet")
		if strings.TrimSpace(stdout) != "ActiveState=active" {
			return fmt.Errorf("Fleet unit not reported as active: %s", stdout)
		}
		stdout, _ = cluster.MemberCommand(m, "systemctl", "show", "--property=Result", "fleet")
		if strings.TrimSpace(stdout) != "Result=success" {
			return fmt.Errorf("Result for fleet unit not reported as success: %s", stdout)
		}
		return nil
	}

	stdout, err := cluster.MemberCommand(m0, "cat", machineIdFile)
	if err != nil {
		t.Fatalf("Failed to get machine-id\nstdout: %s\nerr: %v", stdout, err)
	}
	m0_machine_id := strings.TrimSpace(stdout)

	// If the two machine IDs are different with each other,
	// set the m1's ID to the same one as m0, to intentionally
	// trigger an error case of duplication of machine ID.
	stdout, err = cluster.MemberCommand(m1,
		"echo", m0_machine_id, "|", "sudo", "tee", machineIdFile)
	if err != nil {
		t.Fatalf("Failed to replace machine-id\nstdout: %s\nerr: %v", stdout, err)
	}

	if err := restartFleetService(m1); err != nil {
		t.Fatal(err)
	}

	// fleetd should actually be running, but failing to list machines.
	// So we should expect a specific error after running fleetctl list-machines,
	// like "googlapi: Error 503: fleet server unable to communicate with etcd".
	stdout, stderr, err := cluster.Fleetctl(m1, "list-machines", "--no-legend")
	if err != nil {
		if !strings.Contains(err.Error(), "exit status 1") ||
			!strings.Contains(stderr, "fleet server unable to communicate with etcd") {
			t.Fatalf("m1: Failed to get list of machines. err: %v\nstderr: %s", err, stderr)
		}
		// If both conditions are satisfied, "exit status 1" and
		// "...unable to communicate...", then it's an expected error. PASS.
	} else {
		t.Fatalf("m1: should get an error, but got success.\nstderr: %s", stderr)
	}

	// Trigger another test case of m0's ID getting different from m1's.
	// Then it's expected that m0 and m1 would be working properly with distinct
	// machine IDs, after having restarted fleet.service both on m0 and m1.
	stdout, err = cluster.MemberCommand(m0,
		"echo", util.NewMachineID(), "|", "sudo", "tee", machineIdFile)
	if err != nil {
		t.Fatalf("m0: Failed to replace machine-id\nstdout: %s\nerr: %v", stdout, err)
	}

	// Restart fleet service on m0, and see that it's still working.
	if err := restartFleetService(m0); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err = cluster.Fleetctl(m0, "list-machines", "--no-legend")
	if err != nil {
		t.Fatalf("m0: error: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
}
