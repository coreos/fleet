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

// Load service and discovery units and test whether discovery unit adds itself as a dependency for the service.
func TestInstallUnit(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.Destroy(t)

	// Start with a two-nodes cluster
	members, err := platform.CreateNClusterMembers(cluster, 2)
	if err != nil {
		t.Fatal(err)
	}
	m0 := members[0]
	_, err = cluster.WaitForNMachines(m0, 2)
	if err != nil {
		t.Fatal(err)
	}

	// Load unit files
	stdout, stderr, err := cluster.Fleetctl(m0, "load", "fixtures/units/hello.service", "fixtures/units/discovery.service")
	if err != nil {
		t.Fatalf("Failed loading unit files: \nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}

	checkState := func(match string) bool {
		stdout, _, err := cluster.Fleetctl(m0, "--strict-host-key-checking=false", "ssh", "discovery.service", "systemctl show --property=ActiveState discovery.service")
		if err != nil {
			t.Logf("Failed getting info using remote systemctl: %v", err)
		}
		stdout = strings.TrimSpace(stdout)
		return stdout == fmt.Sprintf("ActiveState=%s", match)
	}

	// Verify that discovery.service unit is loaded but not started
	timeout, err := util.WaitForState(func() bool { return checkState("inactive") })
	if err != nil {
		t.Fatalf("discovery.service unit is not reported as inactive within %v: %v", timeout, err)
	}

	// Start hello.service unit
	stdout, stderr, err = cluster.Fleetctl(m0, "start", "fixtures/units/hello.service")
	if err != nil {
		t.Fatalf("Failed starting unit: \nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}

	// Verify that discovery.service unit was started
	timeout, err = util.WaitForState(func() bool { return checkState("active") })
	if err != nil {
		t.Fatalf("discovery.service unit is not reported as active within %v:\n%v", timeout, err)
	}

	// Stop hello.service unit
	stdout, stderr, err = cluster.Fleetctl(m0, "stop", "fixtures/units/hello.service")
	if err != nil {
		t.Fatalf("Failed stopping unit: \nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}

	// Verify that discovery.service unit was stopped
	timeout, err = util.WaitForState(func() bool { return checkState("inactive") })
	if err != nil {
		t.Fatalf("discovery.service unit is not reported as inactive within %v:\n%v", timeout, err)
	}
}
