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

	if err := cluster.CreateMember("1", platform.MachineConfig{}); err != nil {
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

	if _, _, err := cluster.FleetctlWithInput("yes", "--strict-host-key-checking=true", fmt.Sprintf("--known-hosts-file=%s", khFile), "ssh", machine, "uptime"); err != nil {
		t.Errorf("Unable to SSH into fleet machine: %v", err)
	}

	// Recreation of the cluster simulates a change in the server's host key
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
	if _, _, err := cluster.FleetctlWithInput("yes", "--strict-host-key-checking=true", fmt.Sprintf("--known-hosts-file=%s", khFile), "ssh", machine, "uptime"); err != nil {
		t.Errorf("Unable to SSH into fleet machine: %v", err)
	}

}
