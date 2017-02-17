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

// +build all !dummytest

package functional

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/coreos/fleet/functional/platform"
	"github.com/coreos/fleet/functional/util"
)

// Check that units states do not change on loss of connectivity to etcd.
//
// Note: this only tests the behaviour of the disconnected node;
// but not the reaction of the rest of the cluster,
// nor reconciliation after connectivity is restored.
func TestSingleNodeConnectivityLoss(t *testing.T) {
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

	// Set up some units.
	stateMapping := map[string]struct {
		command          []string
		runState         string
		systemdFileState string
		systemdState     []string
	}{
		"inactive": {[]string{"submit"}, "", "", nil},
		"loaded":   {[]string{"load", "--no-block"}, "inactive", "linked-runtime", nil},
		"launched": {[]string{"start", "--no-block"}, "active", "linked-runtime", []string{"loaded", "active", "running"}},
	}
	createUnits := map[string][]string{}
	expectedUnitFiles := map[string]string{}
	expectedUnitStates := map[string]string{}
	expectedSystemdFiles := map[string]string{}
	expectedSystemdStates := map[string][]string{}
	for _, service := range []string{"single", "global"} {
		for state, mapping := range stateMapping {
			unitName := fmt.Sprintf("%s@%s.service", service, state)
			unitPath := fmt.Sprintf("fixtures/units/%s", unitName)
			createUnits[unitName] = append(mapping.command, unitPath)

			expectedUnitFiles[unitName] = state

			if mapping.runState != "" {
				expectedUnitStates[unitName] = mapping.runState
			}

			if mapping.systemdFileState != "" {
				expectedSystemdFiles[unitName] = mapping.systemdFileState
			}

			if mapping.systemdState != nil {
				expectedSystemdStates[unitName] = mapping.systemdState
			}
		}
	}
	for name, command := range createUnits {
		stdout, stderr, err := cluster.Fleetctl(m0, command...)
		if err != nil {
			t.Fatalf("Failed creating unit %s: %v\nstdout: %s\nstderr:%s", name, err, stdout, stderr)
		}
	}

	checkExpectedStates := func() (isExpected bool, expected, actual map[string]string) {
		// First check unit files.
		// These shouldn't change at all after intital submit -- but better safe than sorry...
		stdout, stderr, err := cluster.Fleetctl(m0, "list-unit-files", "--no-legend", "--full", "--fields", "unit,dstate")
		if err != nil {
			t.Errorf("Failed listing unit files:\nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
		}
		stdout = strings.TrimSpace(stdout)

		lines := strings.Split(stdout, "\n")
		actualUnitFiles := map[string]string{}
		if stdout != "" {
			for _, line := range lines {
				cols := strings.Fields(line)
				actualUnitFiles[cols[0]] = cols[1]
			}
		}

		if !reflect.DeepEqual(actualUnitFiles, expectedUnitFiles) {
			return false, expectedUnitFiles, actualUnitFiles
		}

		// Now check the actual unit states.
		stdout, stderr, err = cluster.Fleetctl(m0, "list-units", "--no-legend", "--full", "--fields", "unit,active")
		if err != nil {
			t.Errorf("Failed listing units:\nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
		}
		stdout = strings.TrimSpace(stdout)

		lines = strings.Split(stdout, "\n")
		actualUnitStates := map[string]string{}
		if stdout != "" {
			for _, line := range lines {
				cols := strings.Fields(line)
				actualUnitStates[cols[0]] = cols[1]
			}
		}

		return reflect.DeepEqual(actualUnitStates, expectedUnitStates), expectedUnitStates, actualUnitStates
	}

	// Wait for initial state being reached.
	timeout, err := util.WaitForState(
		func() bool { isExpected, _, _ := checkExpectedStates(); return isExpected },
	)
	if err != nil {
		t.Fatalf("Failed to reach expected initial state within %v.", timeout)
	}

	// Cut connection to etcd.
	//
	// We use REJECT here, so fleet knows immediately that it's disconnected, rather than waiting for a timeout.
	stdout, stderr, err := cluster.MemberCommand(m0, "sudo", "iptables", "-I", "OUTPUT", "-p", "tcp", "-m", "multiport", "--dports=2379,4001", "-j", "REJECT")
	if err != nil {
		t.Fatalf("Failed to cut connection to etcd.\nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}

	// Wait long enough to be reasonably confident that no more state changes will happen.
	ttl, _ := time.ParseDuration(util.FleetTTL)
	agentReconcileInterval := 5 * time.Second
	slack := 2 * time.Second

	time.Sleep(ttl + agentReconcileInterval + slack)

	// Check unit state after connection loss.
	//
	// Note: we cannot use fleetctl to check the state here,
	// as fleet is not available to give us this information...
	// We have to go deeper, and try to obtain the information from systemd directly.
	actualSystemdFiles := map[string]string{}
	for name, _ := range expectedSystemdFiles {
		stdout, _, _ := cluster.MemberCommand(m0, "systemctl", "is-enabled", name)
		// do not check for error, as systemctl is-enabled returns exit status 1 for linked-runtime.
		stdout = strings.TrimSpace(stdout)
		if stdout == "" {
			continue
		}
		actualSystemdFiles[name] = strings.Split(stdout, "\n")[0]
	}

	if !reflect.DeepEqual(actualSystemdFiles, expectedSystemdFiles) {
		t.Fatalf("Units files not in expected state after losing connectivity.\nExpected: %v\nActual: %v", expectedSystemdFiles, actualSystemdFiles)
	}

	stdout, stderr, err = cluster.MemberCommand(m0, "systemctl", "list-units", "-t", "service", "--no-legend", "single@*.service", "global@*.service")
	if err != nil {
		t.Fatalf("Failed to retrieve systemd unit states.\nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}
	stdout = strings.TrimSpace(stdout)
	actualSystemdStates := map[string][]string{}
	if stdout != "" {
		for _, line := range strings.Split(stdout, "\n") {
			fields := strings.Fields(line)
			actualSystemdStates[fields[0]] = fields[1:4]
		}
	}
	if !reflect.DeepEqual(actualSystemdStates, expectedSystemdStates) {
		t.Fatalf("Units not in expected state after losing connectivity.\nExpected: %v\nActual: %v", expectedSystemdStates, actualSystemdStates)
	}

	// Restore etcd connection.
	if stdout, stderr, err = cluster.MemberCommand(m0, "sudo", "iptables", "-D", "OUTPUT", "-p", "tcp", "-m", "multiport", "--dports=2379,4001", "-j", "REJECT"); err != nil {
		t.Fatalf("Failed to restore etcd connection.\nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}

	// Again, wait long enough to be reasonably confident that no more state changes will happen.
	//
	// Here this should cover the time for fleet to realise connectivity is back,
	// and for the Agent to complete the second run after reconnection.
	//
	// (Unlike for the first run immediately after connectivity is back, by the time of the second run,
	// Engine leadership and Engine reconciliation should have been sorted out,
	// and everything should be back to normal...)
	time.Sleep(ttl + agentReconcileInterval + slack)

	// Check state after reconnect.
	var expected, actual map[string]string
	var isExpected bool
	timeout, err = util.WaitForState(
		func() bool { isExpected, expected, actual = checkExpectedStates(); return isExpected },
	)
	if err != nil {
		t.Fatalf("Failed to reach expected initial state within %v.", timeout)
	}
	if !isExpected {
		t.Fatalf("Units not in expected state after restoring connectivity.\nExpected: %v\nActual: %v", expected, actual)
	}

	// Additionally check the logs of all active units for possible temporary state flapping.
	stdout, stderr, err = cluster.MemberCommand(m0, "journalctl", "_PID=1")
	if err != nil {
		t.Fatalf("Failed to retrieve journal.\nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}
	if strings.Contains(stdout, "Stopping single@") || strings.Contains(stdout, "Stopping global@") {
		t.Fatalf("Units were unexpectedly stopped at some point:\n%s", stdout)
	}
}
