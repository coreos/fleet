/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

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
	defer cluster.Destroy()

	if _, err := cluster.CreateMember("1", platform.MachineConfig{}); err != nil {
		t.Fatal(err)
	}
	machines, err := cluster.WaitForNMachines(1)
	if err != nil {
		t.Fatal(err)
	}
	machine := machines[0]

	tmp, err := ioutil.TempFile(os.TempDir(), "known-hosts")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	defer syscall.Unlink(tmp.Name())

	khFile := tmp.Name()

	if stdout, stderr, err := cluster.FleetctlWithInput("yes", "--strict-host-key-checking=true", fmt.Sprintf("--known-hosts-file=%s", khFile), "ssh", machine, "uptime"); err != nil {
		t.Errorf("Unable to SSH into fleet machine: \nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}

	// Gracefully poweroff the machine to allow fleet to purge its state.
	cluster.PoweroffMember("1")

	machines, err = cluster.WaitForNMachines(0)
	if err != nil {
		t.Fatal(err)
	}

	cluster.DestroyMember("1")
	cluster.CreateMember("1", platform.MachineConfig{})
	machines, err = cluster.WaitForNMachines(1)
	if err != nil {
		t.Fatal(err)
	}
	machine = machines[0]

	// SSH'ing to the cluster member should now fail with a host key mismatch
	if _, _, err := cluster.Fleetctl("--strict-host-key-checking=true", fmt.Sprintf("--known-hosts-file=%s", khFile), "ssh", machine, "uptime"); err == nil {
		t.Errorf("Expected error while SSH'ing to fleet machine")
	}

	// Overwrite the known-hosts file to simulate removing the old host key
	if err := ioutil.WriteFile(khFile, []byte{}, os.FileMode(0644)); err != nil {
		t.Fatalf("Unable to overwrite known-hosts file: %v", err)
	}

	// And SSH should work again
	if stdout, stderr, err := cluster.FleetctlWithInput("yes", "--strict-host-key-checking=true", fmt.Sprintf("--known-hosts-file=%s", khFile), "ssh", machine, "uptime"); err != nil {
		t.Errorf("Unable to SSH into fleet machine: \nstdout: %s\nstderr: %s\nerr: %v", stdout, stderr, err)
	}

}
