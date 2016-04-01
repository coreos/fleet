// Copyright 2016 CoreOS, Inc.
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
)

// Check clean shutdown of fleetd under normal circumstances
func TestShutdown(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.Destroy()

	m0, err := cluster.CreateMember()
	if err != nil {
		t.Fatal(err)
	}
	_, err = cluster.WaitForNMachines(m0, 1)
	if err != nil {
		t.Fatal(err)
	}

	// Stop the fleet process.
	if _, err = cluster.MemberCommand(m0, "sudo", "systemctl", "stop", "fleet"); err != nil {
		t.Fatal(err)
	}

	// Check expected state after stop.
	stdout, _ := cluster.MemberCommand(m0, "systemctl", "show", "--property=ActiveState", "fleet")
	if strings.TrimSpace(stdout) != "ActiveState=inactive" {
		t.Fatalf("Fleet unit not reported as inactive: %s", stdout)
	}
	stdout, _ = cluster.MemberCommand(m0, "systemctl", "show", "--property=Result", "fleet")
	if strings.TrimSpace(stdout) != "Result=success" {
		t.Fatalf("Result for fleet unit not reported as success: %s", stdout)
	}
}

// Check clean shutdown of fleetd while automatic restart (after failed health check) is in progress
func TestShutdownVsMonitor(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.Destroy()

	m0, err := cluster.CreateMember()
	if err != nil {
		t.Fatal(err)
	}
	_, err = cluster.WaitForNMachines(m0, 1)
	if err != nil {
		t.Fatal(err)
	}

	// Cut connection to etcd.
	//
	// This will result in a failed health check, and consequently the monitor will attempt a restart.
	if _, err = cluster.MemberCommand(m0, "sudo", "iptables", "-I", "OUTPUT", "-p", "tcp", "-m", "multiport", "--dports=2379,4001", "-j", "DROP"); err != nil {
		t.Fatal(err)
	}

	// Wait for the monitor to trigger the restart.
	//
	// This will never complete, as long as there is no connectivity.
	if _, err = cluster.MemberCommand(m0, "sudo", "sh", "-c", `'until journalctl -u fleet | grep -q "Server monitor triggered: Monitor timed out before successful heartbeat"; do sleep 1; done'`); err != nil {
		t.Fatal(err)
	}

	// Stop fleetd while the restart is still in progress.
	if _, err = cluster.MemberCommand(m0, "sudo", "systemctl", "stop", "fleet"); err != nil {
		t.Fatal(err)
	}

	// Verify that fleetd was shut down cleanly in spite of the concurrent restart.
	stdout, _ := cluster.MemberCommand(m0, "systemctl", "show", "--property=ActiveState", "fleet")
	if strings.TrimSpace(stdout) != "ActiveState=inactive" {
		t.Fatalf("Fleet unit not reported as inactive: %s", stdout)
	}
	stdout, _ = cluster.MemberCommand(m0, "systemctl", "show", "--property=Result", "fleet")
	if strings.TrimSpace(stdout) != "Result=success" {
		t.Fatalf("Result for fleet unit not reported as success: %s", stdout)
	}
}
