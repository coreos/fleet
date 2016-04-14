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
	"strings"
	"testing"

	"github.com/coreos/fleet/functional/platform"
	"github.com/coreos/fleet/functional/util"
)

const (
	tmpHelloService = "/tmp/hello.service"
	fxtHelloService = "fixtures/units/hello.service"
	tmpFixtures     = "/tmp/fixtures"
	numUnitsSubmit  = 3
)

// TestUnitRunnable is the simplest test possible, deplying a single-node
// cluster and ensuring a unit can enter an 'active' state
func TestUnitRunnable(t *testing.T) {
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

	if stdout, stderr, err := cluster.Fleetctl(m0, "start", "fixtures/units/hello.service"); err != nil {
		t.Fatalf("Unable to start fleet unit: \nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}

	units, err := cluster.WaitForNActiveUnits(m0, 1)
	if err != nil {
		t.Fatal(err)
	}
	_, found := units["hello.service"]
	if len(units) != 1 || !found {
		t.Fatalf("Expected hello.service to be sole active unit, got %v", units)
	}
}

func TestUnitSubmit(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.Destroy()

	m, err := cluster.CreateMember()
	if err != nil {
		t.Fatal(err)
	}
	_, err = cluster.WaitForNMachines(m, 1)
	if err != nil {
		t.Fatal(err)
	}

	err = submitUnitCommon(cluster, m)
	if err != nil {
		t.Fatal(err)
	}

	if err := submitUnitMultiple(cluster, m, numUnitsSubmit); err != nil {
		t.Fatal(err)
	}
}

func submitUnitMultiple(cluster platform.Cluster, m platform.Member, n int) error {
	cmd := "submit"

	if _, err := os.Stat(tmpFixtures); os.IsNotExist(err) {
		os.Mkdir(tmpFixtures, 0755)
	}

	var stdout string
	var err error
	for i := 1; i <= n; i++ {
		tmpHelloFixture := fmt.Sprintf("/tmp/fixtures/hello%d.service", i)

		// copy a file to /tmp
		err = util.CopyFile(tmpHelloFixture, fxtHelloService)
		if err != nil {
			return fmt.Errorf("Failed to copy a temp fleet service: %v", err)
		}

		// run a command for a unit and assert it shows up
		if _, _, err := cluster.Fleetctl(m, cmd, tmpHelloFixture); err != nil {
			return fmt.Errorf("Unable to %s fleet unit: %v", cmd, err)
		}

		stdout, _, err = cluster.Fleetctl(m, "list-unit-files", "--no-legend")
		if err != nil {
			return fmt.Errorf("Failed to run %s: %v", "list-unit-files", err)
		}
		units := strings.Split(strings.TrimSpace(stdout), "\n")
		if len(units) != i {
			return fmt.Errorf("Did not find %d units in cluster: \n%s", i, stdout)
		}
	}

	// All the tests under fixtures are identical, so hash of each unit must be
	// identical. That means, destroying one of them must result in destroying
	// all of them.
	for i := 1; i <= n; i++ {
		tmpHelloFixture := fmt.Sprintf("/tmp/fixtures/hello%d.service", i)

		// destroy a service file
		if _, _, err := cluster.Fleetctl(m, "destroy", tmpHelloFixture); err != nil {
			fmt.Printf("Unable to destroy fleet unit: %v", err)
			continue
		}
		os.Remove(tmpHelloFixture)
	}

	expectedCount := 0
	waitForNUnits := func() bool {
		stdout, _, err := cluster.Fleetctl(m, "list-unit-files", "--no-legend")
		if err != nil {
			return false
		}
		units := strings.Split(strings.TrimSpace(stdout), "\n")
		if (expectedCount == 0 && len(stdout) == 0) || len(units) == expectedCount {
			return true
		}
		return false
	}
	_, err = util.WaitForState(waitForNUnits)
	if err != nil {
		return fmt.Errorf("Failed to get every unit to be cleaned up: %v", err)
	}
	os.Remove(tmpFixtures)

	return nil
}

func submitUnitCommon(cluster platform.Cluster, m platform.Member) error {
	// submit a unit and assert it shows up
	if _, _, err := cluster.Fleetctl(m, "submit", "fixtures/units/hello.service"); err != nil {
		fmt.Errorf("Unable to submit fleet unit: %v", err)
	}
	stdout, _, err := cluster.Fleetctl(m, "list-unit-files", "--no-legend")
	if err != nil {
		fmt.Errorf("Failed to run list-unit-files: %v", err)
	}
	units := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(units) != 1 {
		fmt.Errorf("Did not find 1 unit in cluster: \n%s", stdout)
	}

	// submitting the same unit should not fail
	if _, _, err = cluster.Fleetctl(m, "submit", "fixtures/units/hello.service"); err != nil {
		fmt.Errorf("Expected no failure when double-submitting unit, got this: %v", err)
	}

	// destroy the unit and ensure it disappears from the unit list
	if _, _, err := cluster.Fleetctl(m, "destroy", "fixtures/units/hello.service"); err != nil {
		fmt.Errorf("Failed to destroy unit: %v", err)
	}

	expectedCount := 0
	waitForNUnits := func() bool {
		stdout, _, err := cluster.Fleetctl(m, "list-unit-files", "--no-legend")
		if err != nil {
			return false
		}
		units := strings.Split(strings.TrimSpace(stdout), "\n")
		if (expectedCount == 0 && len(stdout) == 0) || len(units) == expectedCount {
			return true
		}
		return false
	}
	_, err = util.WaitForState(waitForNUnits)
	if err != nil {
		fmt.Errorf("Failed to get every unit to be cleaned up: %v", err)
	}

	// submitting the unit after destruction should succeed
	if _, _, err := cluster.Fleetctl(m, "submit", "fixtures/units/hello.service"); err != nil {
		fmt.Errorf("Unable to submit fleet unit: %v", err)
	}
	stdout, _, err = cluster.Fleetctl(m, "list-unit-files", "--no-legend")
	if err != nil {
		fmt.Errorf("Failed to run list-unit-files: %v", err)
	}
	units = strings.Split(strings.TrimSpace(stdout), "\n")
	if len(units) != 1 {
		fmt.Errorf("Did not find 1 unit in cluster: \n%s", stdout)
	}

	// destroy the unit again
	if _, _, err := cluster.Fleetctl(m, "destroy", "fixtures/units/hello.service"); err != nil {
		fmt.Errorf("Failed to destroy unit: %v", err)
	}

	_, err = util.WaitForState(waitForNUnits)
	if err != nil {
		fmt.Errorf("Failed to get every unit to be cleaned up: %v", err)
	}

	return nil
}

func TestUnitRestart(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.Destroy()

	m, err := cluster.CreateMember()
	if err != nil {
		t.Fatal(err)
	}
	_, err = cluster.WaitForNMachines(m, 1)
	if err != nil {
		t.Fatal(err)
	}

	if stdout, stderr, err := cluster.Fleetctl(m, "start", "fixtures/units/hello.service"); err != nil {
		t.Fatalf("Unable to start fleet unit: \nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}

	units, err := cluster.WaitForNActiveUnits(m, 1)
	if err != nil {
		t.Fatal(err)
	}
	_, found := units["hello.service"]
	if len(units) != 1 || !found {
		t.Fatalf("Expected hello.service to be sole active unit, got %v", units)
	}

	if _, _, err := cluster.Fleetctl(m, "stop", "hello.service"); err != nil {
		t.Fatal(err)
	}
	units, err = cluster.WaitForNActiveUnits(m, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 0 {
		t.Fatalf("Zero units should be running, found %v", units)
	}

	if stdout, stderr, err := cluster.Fleetctl(m, "start", "hello.service"); err != nil {
		t.Fatalf("Unable to start fleet unit: \nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}
	units, err = cluster.WaitForNActiveUnits(m, 1)
	if err != nil {
		t.Fatal(err)
	}
	_, found = units["hello.service"]
	if len(units) != 1 || !found {
		t.Fatalf("Expected hello.service to be sole active unit, got %v", units)
	}

}

func TestUnitSSHActions(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.Destroy()

	m, err := cluster.CreateMember()
	if err != nil {
		t.Fatal(err)
	}
	_, err = cluster.WaitForNMachines(m, 1)
	if err != nil {
		t.Fatal(err)
	}

	if stdout, stderr, err := cluster.Fleetctl(m, "start", "--no-block", "fixtures/units/hello.service"); err != nil {
		t.Fatalf("Unable to start fleet unit: \nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}

	units, err := cluster.WaitForNActiveUnits(m, 1)
	if err != nil {
		t.Fatal(err)
	}

	_, found := units["hello.service"]
	if len(units) != 1 || !found {
		t.Fatalf("Expected hello.service to be sole active unit, got %v", units)
	}

	stdout, stderr, err := cluster.Fleetctl(m, "--strict-host-key-checking=false", "ssh", "hello.service", "echo", "foo")
	if err != nil {
		t.Errorf("Failure occurred while calling fleetctl ssh: %v\nstdout: %v\nstderr: %v", err, stdout, stderr)
	}

	if !strings.Contains(stdout, "foo") {
		t.Errorf("Could not find expected string in command output:\n%s", stdout)
	}

	stdout, stderr, err = cluster.Fleetctl(m, "--strict-host-key-checking=false", "status", "hello.service")
	if err != nil {
		t.Errorf("Failure occurred while calling fleetctl status: %v\nstdout: %v\nstderr: %v", err, stdout, stderr)
	}

	if !strings.Contains(stdout, "Active: active") {
		t.Errorf("Could not find expected string in status output:\n%s", stdout)
	}

	stdout, stderr, err = cluster.Fleetctl(m, "--strict-host-key-checking=false", "journal", "--sudo", "hello.service")
	if err != nil {
		t.Errorf("Failure occurred while calling fleetctl journal: %v\nstdout: %v\nstderr: %v", err, stdout, stderr)
	}

	if !strings.Contains(stdout, "Hello, World!") {
		t.Errorf("Could not find expected string in journal output:\n%s", stdout)
	}
}
