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
	"path"
	"regexp"
	"strings"
	"testing"

	"github.com/coreos/fleet/functional/platform"
	"github.com/coreos/fleet/functional/util"
)

// Start three machines and test template units based on machines Metadata
func TestTemplatesWithSpecifiersInMetadata(t *testing.T) {
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
	_, err = cluster.WaitForNMachines(m0, 3)
	if err != nil {
		t.Fatal(err)
	}

	// Submit one template
	if stdout, stderr, err := cluster.Fleetctl(m0, "submit", "fixtures/units/metadata@.service"); err != nil {
		t.Fatalf("Unable to submit metadata@.service template: \nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}

	// Start units based on template in backward order
	for i := len(members) - 1; i >= 0; i-- {
		if stdout, stderr, err := cluster.Fleetctl(m0, "start", fmt.Sprintf("fixtures/units/metadata@smoke%s.service", members[i].ID())); err != nil {
			t.Fatalf("Unable to start template based unit: \nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
		}
	}

	_, err = cluster.WaitForNActiveUnits(m0, 3)
	if err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := cluster.Fleetctl(m0, "list-units", "--no-legend", "--full", "--fields", "unit,active,machine")
	if err != nil {
		t.Fatalf("Unable to get submitted units: \nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}

	ndesired := 3
	stdout = strings.TrimSpace(stdout)
	lines := strings.Split(stdout, "\n")
	allStates := util.ParseUnitStates(lines)
	active := util.FilterActiveUnits(allStates)
	nactive := len(active)
	if nactive != ndesired {
		t.Fatalf("Failed to get %d active units: \nstdout: %s\nstderr: %s", ndesired, stdout, stderr)
	}

	for _, state := range active {
		re := regexp.MustCompile(`@([^.]*)`)
		desiredMachine := re.FindStringSubmatch(state.Name)
		if len(desiredMachine) < 2 {
			t.Fatalf("Cannot parse state.Name (%v): \nstdout: %s\nstderr: %s", state.Name, stdout, stderr)
		}
		currentMachine := fmt.Sprintf("smoke%s", state.Machine)
		if desiredMachine[1] != currentMachine {
			t.Fatalf("Template (%s) has been scheduled on wrong machine (%s): \nstdout: %s\nstderr: %s", state.Name, currentMachine, stdout, stderr)
		}
	}

	if stdout, stderr, err := cluster.Fleetctl(m0, "start", "--block-attempts=20", "fixtures/units/metadata@invalid.service"); err == nil {
		t.Fatalf("metadata@invalid unit should not be scheduled: \nstdout: %s\nstderr: %s", stdout, stderr)
	}
}

// TestMetadataOperator ensures that metadata operators work also for
// extended operators such as ">=", "<=", "<", ">", "!=", or "==".
func TestMetadataOperator(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.Destroy(t)

	members, err := platform.CreateNClusterMembers(cluster, 1)
	if err != nil {
		t.Fatal(err)
	}
	m0 := members[0]
	_, err = cluster.WaitForNMachines(m0, 1)
	if err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := cluster.Fleetctl(m0, "list-machines", "--fields", "machine,metadata")
	if err != nil {
		t.Fatalf("Unable to get machine metadata\nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}

	runMetaOp := func(ramEq string, expectSuccess bool) {
		tmpMdOpService := "/tmp/metadata-op.service"
		MdOpService := "fixtures/units/metadata-op.service"
		MdOpBaseName := path.Base(MdOpService)
		var nUnits int

		if expectSuccess {
			t.Logf("Testing %s expecting success...", ramEq)
			nUnits = 1
		} else {
			t.Logf("Testing %s expecting failure...", ramEq)
			nUnits = 0
		}

		err = util.GenNewFleetService(tmpMdOpService, MdOpService, ramEq, "ram>=1024")
		if err != nil {
			t.Fatalf("Failed to generate a temp fleet service: %v", err)
		}

		stdout, stderr, err = cluster.Fleetctl(m0, "start", "--no-block", tmpMdOpService)
		if err != nil {
			t.Fatalf("starting unit %s returned error:\nstdout: %s\nstderr: %s\nerr: %v",
				tmpMdOpService, stdout, stderr, err)
		}

		_, err = cluster.WaitForNActiveUnits(m0, nUnits)
		if err != nil {
			t.Fatal(err)
		}

		stdout, stderr, err = cluster.Fleetctl(m0, "destroy", MdOpBaseName)
		if err != nil {
			t.Fatalf("unit %s cannot be stopped: \nstdout: %s\nstderr: %s\nerr: %v",
				MdOpBaseName, stdout, stderr, err)
		}

		_, err = cluster.WaitForNUnitFiles(m0, 0)
		if err != nil {
			t.Fatal(err)
		}

	}

	// run tests for success cases
	runMetaOp("ram>=1024", true)
	runMetaOp("ram<=1024", true)
	runMetaOp("ram>1023", true)
	runMetaOp("ram<1025", true)
	runMetaOp("ram!=1025", true)
	runMetaOp("ram==1024", true)

	// run tests for failure cases
	runMetaOp("ram>=1025", false)
	runMetaOp("ram<=1023", false)
	runMetaOp("ram>1024", false)
	runMetaOp("ram<1024", false)
	runMetaOp("ram!=1024", false)
	runMetaOp("ram==1025", false)
}
