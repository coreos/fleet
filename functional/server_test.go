// Copyright 2016 The fleet Authors
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

// TestReconfigureServer checks whether fleetd managed to keep its listeners
// across reconfiguration of fleetd after receiving SIGHUP.
func TestReconfigureServer(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.Destroy(t)

	m0, err := cluster.CreateMember()
	if err != nil {
		t.Fatal(err)
	}
	_, err = cluster.WaitForNMachines(m0, 1)
	if err != nil {
		t.Fatal(err)
	}

	err = waitForFleetdSocket(cluster, m0)
	if err != nil {
		t.Fatalf("Failed to get a list of fleetd sockets: %v", err)
	}

	unit := fmt.Sprintf("fixtures/units/hello.service")
	stdout, stderr, err := cluster.Fleetctl(m0, "start", unit)
	if err != nil {
		t.Fatalf("Failed starting unit: \nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}

	_, err = cluster.WaitForNActiveUnits(m0, 1)
	if err != nil {
		t.Fatal(err)
	}

	// Trigger AgentReconciler just here
	stdout, stderr, err = cluster.Fleetctl(m0, "unload", unit)
	if err != nil {
		t.Fatalf("Failed unloading unit: \nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}

	// Send a SIGHUP to fleetd, and periodically checks if a message
	// "Reloading configuration" appears in fleet's journal, up to timeout (15) seconds.
	stdout, _ = cluster.MemberCommand(m0, "sudo", "systemctl", "kill", "-s", "SIGHUP", "fleet")
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("Sending SIGHUP to fleetd returned: %s", stdout)
	}

	// Watch the logs if fleet was correctly reloaded
	errSigHup := waitForReloadConfig(cluster, m0)
	if errSigHup != nil {
		t.Logf("Failed to ensure that fleet was correctly reloaded: %v", errSigHup)
	}

	// check if fleetd is still running correctly, by running fleetctl status
	// Even if the log message do not show up this test may catch the error.
	stdout, stderr, err = cluster.Fleetctl(m0, "list-units")
	if err != nil {
		t.Fatalf("Unable to check list-units. Please check for fleetd socket\nstdout: %s\nstderr: %s\nerr:%v",
			stdout, stderr, err)
	}

	// Ensure that fleet received SIGHUP, if not then just skip this test
	// probably due to journald and or other delays.
	if errSigHup != nil {
		err = waitForReloadConfig(cluster, m0)
		if err != nil {
			// Just mark the test skipped since it did not fail, previous
			// list-units command did succeed. Missing logs can be caused
			// by journald delays or any other race.
			t.Skipf("Skipping Test: Failed to ensure that fleet was correctly reloaded: %v", err)
		}
	}

	// Check for HTTP listener error looking into the fleetd journal
	stdout, _ = cluster.MemberCommand(m0, "journalctl _PID=$(pidof fleetd)")
	if strings.Contains(strings.TrimSpace(stdout), "Failed serving HTTP on listener:") {
		t.Fatalf("Fleetd log returned error on HTTP listeners: %s", stdout)
	}

	// Check expected state after reconfiguring fleetd
	stdout, _ = cluster.MemberCommand(m0, "systemctl", "show", "--property=ActiveState", "fleet")
	if strings.TrimSpace(stdout) != "ActiveState=active" {
		t.Fatalf("Fleet unit not reported as active: %s", stdout)
	}
	stdout, _ = cluster.MemberCommand(m0, "systemctl", "show", "--property=Result", "fleet")
	if strings.TrimSpace(stdout) != "Result=success" {
		t.Fatalf("Result for fleet unit not reported as success: %s", stdout)
	}
}

// waitForReloadConfig returns if a message "Reloading configuration" exists
// in the journal, periodically checking for the journal up to the timeout.
func waitForReloadConfig(cluster platform.Cluster, m0 platform.Member) (err error) {
	_, err = util.WaitForState(
		func() bool {
			// NOTE: journalctl should run just simply like "journalctl -u fleet",
			// without being piped with grep. Doing
			// "journalctl -u fleet | grep \"Reloading configuration\"" is racy
			// in a subtle way, so that it sometimes fails only on semaphoreci.
			// - dpark 20160408
			stdout, _ := cluster.MemberCommand(m0, "sudo", "journalctl --priority=info _PID=$(pidof fleetd)")
			journalfleet := strings.TrimSpace(stdout)
			if !strings.Contains(journalfleet, "Reloading configuration") {
				fmt.Errorf("Fleetd is not fully reconfigured, retrying... entire fleet journal:\n%v", journalfleet)
				return false
			}
			return true
		},
	)
	if err != nil {
		return fmt.Errorf("Reloading configuration log not found: %v", err)
	}

	return nil
}

// waitForFleetdSocket returns if /var/run/fleet.sock exists, periodically
// checking for states.
func waitForFleetdSocket(cluster platform.Cluster, m0 platform.Member) (err error) {
	_, err = util.WaitForState(
		func() bool {
			stdout, _ := cluster.MemberCommand(m0, "test -S /var/run/fleet.sock && echo 1")
			if strings.TrimSpace(stdout) == "" {
				fmt.Errorf("Fleetd is not fully started, retrying...")
				return false
			}
			return true
		},
	)
	if err != nil {
		return fmt.Errorf("Fleetd socket not found: %v", err)
	}

	return nil
}
