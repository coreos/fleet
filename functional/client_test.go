// Copyright 2014 The fleet Authors
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
	"os"
	"syscall"
	"testing"

	"github.com/coreos/fleet/functional/platform"
)

func TestKnownHostsVerification(t *testing.T) {
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

	tmp, err := ioutil.TempFile(os.TempDir(), "known-hosts")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	defer syscall.Unlink(tmp.Name())

	khFile := tmp.Name()

	if stdout, stderr, err := cluster.FleetctlWithInput(m0, "yes", "--strict-host-key-checking=true", fmt.Sprintf("--known-hosts-file=%s", khFile), "ssh", m1.ID(), "uptime"); err != nil {
		t.Errorf("Unable to SSH into fleet machine: \nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}

	m1, err = cluster.ReplaceMember(members[1])
	if err != nil {
		t.Fatalf("Failed replacing machine: %v", err)
	}

	_, err = cluster.WaitForNMachines(m0, 2)
	if err != nil {
		t.Fatal(err)
	}

	// SSH'ing to the cluster member should now fail with a host key mismatch
	if stdout, stderr, err := cluster.Fleetctl(m0, "--strict-host-key-checking=true", fmt.Sprintf("--known-hosts-file=%s", khFile), "ssh", m1.ID(), "uptime"); err == nil {
		t.Errorf("Expected error while SSH'ing to fleet machine\nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}

	// Overwrite the known-hosts file to simulate removing the old host key
	if err := ioutil.WriteFile(khFile, []byte{}, os.FileMode(0644)); err != nil {
		t.Fatalf("Unable to overwrite known-hosts file: %v", err)
	}

	// And SSH should work again
	if stdout, stderr, err := cluster.FleetctlWithInput(m0, "yes", "--strict-host-key-checking=true", fmt.Sprintf("--known-hosts-file=%s", khFile), "ssh", m1.ID(), "uptime"); err != nil {
		t.Errorf("Unable to SSH into fleet machine: \nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}

}

// TestFleetctlWithEnv runs simply fleetctl list-machines, but by setting an
// environment variable FLEETCTL_ENDPOINT, instead of the cmdline option
// '--endpoint'.
func TestFleetctlWithEnv(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.Destroy(t)

	m, err := cluster.CreateMember()
	if err != nil {
		t.Fatal(err)
	}

	_, err = cluster.WaitForNMachines(m, 1)
	if err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := cluster.FleetctlWithEnv(m, "list-machines")
	if err != nil {
		t.Fatalf("Failed to run with env var FLEETCTL_ENDPOINT:\nstdout: %s\nstderr: %s\nerr: %v",
			stdout, stderr, err)
	}
}
