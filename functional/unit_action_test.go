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
	"io/ioutil"
	"path"
	"strings"
	"testing"

	"github.com/coreos/fleet/functional/platform"
	"github.com/coreos/fleet/functional/util"
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

// TestUnitSubmit checks if a unit becomes submitted and destroyed successfully.
// First it submits a unit, and destroys the unit, verifies it's destroyed,
// finally submits the unit again.
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

	if err := doMultipleUnitsCmd(cluster, m, "submit", 9); err != nil {
		t.Fatal(err)
	}
}

// TestUnitLoad checks if a unit becomes loaded and unloaded successfully.
// First it load a unit, and unloads the unit, verifies it's unloaded,
// finally loads the unit again.
func TestUnitLoad(t *testing.T) {
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

	if err := doMultipleUnitsCmd(cluster, m, "load", 6); err != nil {
		t.Fatal(err)
	}
}

func TestUnitStart(t *testing.T) {
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

	if err := doMultipleUnitsCmd(cluster, m, "start", 3); err != nil {
		t.Fatal(err)
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

// TestUnitCat simply compares body of a unit file with that of a unit fetched
// from the remote cluster using "fleetctl cat".
func TestUnitCat(t *testing.T) {
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

	// read a sample unit file to a buffer
	unitFile := "fixtures/units/hello.service"
	fileBuf, err := ioutil.ReadFile(unitFile)
	if err != nil {
		t.Fatal(err)
	}
	fileBody := strings.TrimSpace(string(fileBuf))

	// submit a unit and assert it shows up
	_, _, err = cluster.Fleetctl(m, "submit", unitFile)
	if err != nil {
		t.Fatalf("Unable to submit fleet unit: %v", err)
	}
	// wait until the unit gets submitted up to 15 seconds
	_, err = cluster.WaitForNUnitFiles(m, 1)
	if err != nil {
		t.Fatalf("Failed to run list-units: %v", err)
	}

	// cat the unit file and compare it with the original unit body
	stdout, _, err := cluster.Fleetctl(m, "cat", path.Base(unitFile))
	if err != nil {
		t.Fatalf("Unable to submit fleet unit: %v", err)
	}
	catBody := strings.TrimSpace(stdout)

	if strings.Compare(catBody, fileBody) != 0 {
		t.Fatalf("unit body changed across fleetctl cat: \noriginal:%s\nnew:%s", fileBody, catBody)
	}
}

// TestUnitStatus simply checks "fleetctl status hello.service" actually works.
func TestUnitStatus(t *testing.T) {
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

	unitFile := "fixtures/units/hello.service"

	// Load a unit and print out status.
	// Without loading a unit, it's impossible to run fleetctl status
	_, _, err = cluster.Fleetctl(m, "load", unitFile)
	if err != nil {
		t.Fatalf("Unable to load a fleet unit: %v", err)
	}

	// wait until the unit gets loaded up to 15 seconds
	_, err = cluster.WaitForNUnits(m, 1)
	if err != nil {
		t.Fatalf("Failed to run list-units: %v", err)
	}

	stdout, stderr, err := cluster.Fleetctl(m,
		"--strict-host-key-checking=false", "status", path.Base(unitFile))
	if !strings.Contains(stdout, "Loaded: loaded") {
		t.Errorf("Could not find expected string in status output:\n%s\nstderr:\n%s",
			stdout, stderr)
	}
}

func doMultipleUnitsCmd(cluster platform.Cluster, m platform.Member, cmd string, numUnits int) error {
	launchUnitsCmd := func(cmd string, numUnits int) (unitFiles []string, err error) {
		args := []string{cmd}
		for i := 0; i < numUnits; i++ {
			unitFile := fmt.Sprintf("fixtures/units/hello@%d.service", i+1)
			args = append(args, unitFile)
			unitFiles = append(unitFiles, path.Base(unitFile))
		}

		if stdout, stderr, err := cluster.Fleetctl(m, args...); err != nil {
			return nil,
				fmt.Errorf("Unable to %s batch of units: \nstdout: %s\nstderr: %s\nerr: %v",
					cmd, stdout, stderr, err)
		} else if strings.Contains(stderr, "Error") {
			return nil,
				fmt.Errorf("Failed to correctly %s batch of units: \nstdout: %s\nstderr: %s\nerr: %v",
					cmd, stdout, stderr, err)
		}

		return unitFiles, nil
	}

	checkListUnits := func(cmd string, unitFiles []string, inNumUnits int) (err error) {
		// wait until the unit gets processed up to 15 seconds
		if cmd == "submit" {
			listUnitStates, err := cluster.WaitForNUnitFiles(m, inNumUnits)
			if err != nil {
				return fmt.Errorf("Failed to run list-unit-files: %v", err)
			}

			if inNumUnits == 0 && len(listUnitStates) != 0 {
				return fmt.Errorf("Expected nil unit file list, got %v", listUnitStates)
			}

			// given unit name must be there in list-unit-files
			for i := 0; i < inNumUnits; i++ {
				_, found := listUnitStates[unitFiles[i]]
				if len(listUnitStates) != inNumUnits || !found {
					return fmt.Errorf("Expected %s to be unit file, got %v",
						unitFiles[i], listUnitStates)
				}
			}
		} else {
			// cmd == "load" or "start"
			var listUnitStates map[string][]util.UnitState
			if cmd == "load" {
				listUnitStates, err = cluster.WaitForNUnits(m, inNumUnits)
			} else {
				listUnitStates, err = cluster.WaitForNActiveUnits(m, inNumUnits)
			}
			if err != nil {
				return fmt.Errorf("Failed to run list-unit-files: %v", err)
			}

			if inNumUnits == 0 && len(listUnitStates) != 0 {
				return fmt.Errorf("Expected nil unit file list, got %v", listUnitStates)
			}

			// given unit name must be there in list-unit-files
			for i := 0; i < inNumUnits; i++ {
				_, found := listUnitStates[unitFiles[i]]
				if len(listUnitStates) != inNumUnits || !found {
					return fmt.Errorf("Expected %s to be unit file, got %v",
						unitFiles[i], listUnitStates)
				}
			}
		}

		return nil
	}

	destroyUnits := func(dcmd string, unitFiles []string, numUnits int) (err error) {
		for i := 0; i < numUnits; i++ {
			if _, _, err := cluster.Fleetctl(m, dcmd, unitFiles[i]); err != nil {
				return fmt.Errorf("Failed to %s unit: %v", dcmd, err)
			}
		}
		return nil
	}

	dcmd := make(map[string]string, 0)
	dcmd["submit"] = "destroy"
	dcmd["load"] = "unload"
	dcmd["start"] = "stop"

	// launch a batch of processing units
	unitFiles, err := launchUnitsCmd(cmd, numUnits)
	if err != nil {
		return err
	}
	if err := checkListUnits(cmd, unitFiles, numUnits); err != nil {
		return err
	}

	// destroy the unit and ensure it disappears from the unit list
	if err := destroyUnits(dcmd[cmd], unitFiles, numUnits); err != nil {
		return err
	}
	if err := checkListUnits(cmd, unitFiles, 0); err != nil {
		return err
	}

	// launch a batch of processing units
	unitFiles, err = launchUnitsCmd(cmd, numUnits)
	if err != nil {
		return err
	}
	if err := checkListUnits(cmd, unitFiles, numUnits); err != nil {
		return err
	}

	// destroy the unit again, not to affect the next tests for multiple units
	if err := destroyUnits(dcmd[cmd], unitFiles, numUnits); err != nil {
		return err
	}
	if err := checkListUnits(cmd, unitFiles, 0); err != nil {
		return err
	}

	return nil
}
