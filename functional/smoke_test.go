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

func TestSmoke(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatalf(err.Error())
	}
	defer cluster.DestroyAll()

	// Start with a simple three-node cluster
	if err := cluster.CreateMultiple(3, platform.MachineConfig{}); err != nil {
		t.Fatalf(err.Error())
	}
	machines, err := waitForNMachines(3)
	if err != nil {
		t.Fatalf(err.Error())
	}

	// Ensure we can SSH into each machine using fleetctl
	for _, machine := range machines {
		if _, _, err := fleetctl("--strict-host-key-checking=false", "ssh", machine, "uptime"); err != nil {
			t.Errorf("Unable to SSH into fleet machine: %v", err)
		}
	}

	// Start the 5 services
	for i := 0; i < 5; i++ {
		unitName := fmt.Sprintf("fixtures/units/conflict.%d.service", i)
		_, _, err := fleetctl("start", "--no-block", unitName)
		if err != nil {
			t.Errorf("Failed starting %s: %v", unitName, err)
		}
	}

	// All 5 services should be visible immediately and become ACTIVE
	// shortly thereafter
	stdout, _, err := fleetctl("list-units", "--no-legend")
	if err != nil {
		t.Fatalf("Failed to run list-units: %v", err)
	}
	units := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(units) != 5 {
		t.Fatalf("Did not find five units in cluster: \n%s", stdout)
	}
	if _, err := waitForNActiveUnits(3); err != nil {
		t.Fatalf(err.Error())
	}

	// Add two more machines to the cluster and ensure the remaining
	// unscheduled services are picked up.
	if err := cluster.CreateMultiple(2, platform.MachineConfig{}); err != nil {
		t.Fatalf(err.Error())
	}
	machines, err = waitForNMachines(5)
	if err != nil {
		t.Fatalf(err.Error())
	}
	if _, err := waitForNActiveUnits(5); err != nil {
		t.Fatalf(err.Error())
	}
}

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

func TestSignedRequests(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatalf(err.Error())
	}
	defer cluster.DestroyAll()

	cfg := platform.MachineConfig{VerifyUnits: true}
	if err := cluster.CreateMultiple(1, cfg); err != nil {
		t.Fatalf(err.Error())
	}
	_, err = waitForNMachines(1)
	if err != nil {
		t.Fatalf(err.Error())
	}

	_, _, err = fleetctl("start", "--no-block", "--sign=false", "fixtures/units/hello.service")
	if err != nil {
		t.Fatalf("Failed starting hello.service: %v", err)
	}

	_, _, err = fleetctl("start", "--no-block", "--sign=true", "fixtures/units/goodbye.service")
	if err != nil {
		t.Fatalf("Failed starting goodbye.service: %v", err)
	}

	units, err := waitForNActiveUnits(1)
	if err != nil {
		t.Fatalf(err.Error())
	}

	if len(units) != 1 || units[0] != "goodbye.service" {
		t.Fatalf("Expected goodbye.service to be sole active unit, got %v", units)
	}
}
