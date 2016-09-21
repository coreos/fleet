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
	"io/ioutil"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/coreos/fleet/functional/platform"
)

// Start three units using ssh tunnel
func TestTunnelScheduleBatchUnits(t *testing.T) {
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

	tmp, err := ioutil.TempFile(os.TempDir(), "known-hosts")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	defer syscall.Unlink(tmp.Name())

	khFile := tmp.Name()

	// Launch one unit
	if stdout, stderr, err := cluster.FleetctlWithInput(m0, "yes",
		fmt.Sprintf("--tunnel=%s", m0.IP()),
		"--strict-host-key-checking=true",
		fmt.Sprintf("--known-hosts-file=%s", khFile),
		"start",
		"fixtures/units/hello.service"); err != nil {
		t.Fatalf("Unable to submit one unit using ssh tunnel: \nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	} else if strings.Contains(stderr, "Error") {
		t.Fatalf("Failed to correctly submit unit using ssh tunnel: \nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}

	// Combine all parameters and units in one args slice
	args := []string{
		fmt.Sprintf("--tunnel=%s", m0.IP()),
		"--strict-host-key-checking=true",
		fmt.Sprintf("--known-hosts-file=%s", khFile),
		"start",
	}
	for i := 1; i <= 10; i++ {
		args = append(args, fmt.Sprintf("fixtures/units/hello@%d.service", i))
	}

	// Launch a batch of units
	if stdout, stderr, err := cluster.Fleetctl(m0, args...); err != nil {
		t.Fatalf("Unable to submit batch of units using ssh tunnel: \nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	} else if strings.Contains(stderr, "Error") {
		t.Fatalf("Failed to correctly submit batch of units using ssh tunnel: \nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}

	_, err = cluster.WaitForNActiveUnits(m0, 11)
	if err != nil {
		t.Fatal(err)
	}
}
