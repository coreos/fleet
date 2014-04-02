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

func TestKnownHostsVerification(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatalf(err.Error())
	}
	defer cluster.DestroyAll()

	if err := cluster.CreateMultiple(1, platform.MachineConfig{}); err != nil {
		t.Fatalf(err.Error())
	}
	machines, err := waitForNMachines(1)
	if err != nil {
		t.Fatalf(err.Error())
	}
	machine := machines[0]

	tmp, err := ioutil.TempFile(os.TempDir(), "known-hosts")
	if err != nil {
		t.Fatalf(err.Error())
	}
	tmp.Close()
	defer syscall.Unlink(tmp.Name())

	khFile := tmp.Name()

	if _, _, err := fleetctlWithInput("yes", "--strict-host-key-checking=true", fmt.Sprintf("--known-hosts-file=%s", khFile), "ssh", machine, "uptime"); err != nil {
		t.Errorf("Unable to SSH into fleet machine: %v", err)
	}

	// Recreation of the cluster simulates a change in the server's host key
	cluster.DestroyAll()
	cluster.CreateMultiple(1, platform.MachineConfig{})
	machines, err = waitForNMachines(1)
	if err != nil {
		t.Fatalf(err.Error())
	}
	machine = machines[0]

	// SSH'ing to the cluster member should now fail with a host key mismatch
	if _, _, err := fleetctl("--strict-host-key-checking=true", fmt.Sprintf("--known-hosts-file=%s", khFile), "ssh", machine, "uptime"); err == nil {
		t.Errorf("Expected error while SSH'ing to fleet machine")
	}

	// Overwrite the known-hosts file to simulate removing the old host key
	if err := ioutil.WriteFile(khFile, []byte{}, os.FileMode(0644)); err != nil {
		t.Fatalf("Unable to overwrite known-hosts file: %v", err)
	}

	// And SSH should work again
	if _, _, err := fleetctlWithInput("yes", "--strict-host-key-checking=true", fmt.Sprintf("--known-hosts-file=%s", khFile), "ssh", machine, "uptime"); err != nil {
		t.Errorf("Unable to SSH into fleet machine: %v", err)
	}

}

func TestSSHActions(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.DestroyAll()

	if err := cluster.CreateMultiple(1, platform.MachineConfig{}); err != nil {
		t.Fatal(err)
	}
	_, err = waitForNMachines(1)
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := fleetctl("start", "--no-block", "fixtures/units/hello.service"); err != nil {
		t.Fatalf("Unable to start fleet unit: %v", err)
	}

	units, err := waitForNActiveUnits(1)
	if err != nil {
		t.Fatal(err)
	}

	if len(units) != 1 || units[0] != "hello.service" {
		t.Fatalf("Expected hello.service to be sole active unit, got %v", units)
	}

	stdout, _, err := fleetctl("--strict-host-key-checking=false", "ssh", "hello.service", "echo", "foo")
	if err != nil {
		t.Errorf("Failure occurred while calling fleetctl ssh: %v", err)
	}

	if !strings.Contains(stdout, "foo") {
		t.Errorf("Could not find expected string in command output:\n%s", stdout)
	}

	stdout, _, err = fleetctl("--strict-host-key-checking=false", "status", "hello.service")
	if err != nil {
		t.Errorf("Failure occurred while calling fleetctl status: %v", err)
	}

	if !strings.Contains(stdout, "Active: active") {
		t.Errorf("Could not find expected string in status output:\n%s", stdout)
	}

	stdout, _, err = fleetctl("--strict-host-key-checking=false", "journal", "hello.service")
	if err != nil {
		t.Errorf("Failure occurred while calling fleetctl journal: %v", err)
	}

	if !strings.Contains(stdout, "Hello, World!") {
		t.Errorf("Could not find expected string in journal output:\n%s", stdout)
	}
}
