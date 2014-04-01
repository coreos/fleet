package functional

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/coreos/fleet/functional/platform"
)

var fleetctlBinPath string

func init() {
	fleetctlBinPath = os.Getenv("FLEETCTL_BIN")
	if fleetctlBinPath == "" {
		fmt.Println("FLEETCTL_BIN environment variable must be set")
		os.Exit(1)
	} else if _, err := os.Stat(fleetctlBinPath); err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}

	if os.Getenv("SSH_AUTH_SOCK") == "" {
		fmt.Println("SSH_AUTH_SOCK environment variable must be set")
		os.Exit(1)
	}
}

func TestCluster(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatalf(err.Error())
	}
	defer cluster.DestroyAll()

	// Start with a simple three-node cluster
	if err := cluster.Create(3); err != nil {
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
	if err := waitForNActiveUnits(3); err != nil {
		t.Fatalf(err.Error())
	}

	// Add two more machines to the cluster and ensure the remaining
	// unscheduled services are picked up.
	if err := cluster.Create(2); err != nil {
		t.Fatalf(err.Error())
	}
	machines, err = waitForNMachines(5)
	if err != nil {
		t.Fatalf(err.Error())
	}
	if err := waitForNActiveUnits(5); err != nil {
		t.Fatalf(err.Error())
	}
}

func TestKnownHostsVerification(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatalf(err.Error())
	}
	defer cluster.DestroyAll()

	if err := cluster.Create(1); err != nil {
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
	cluster.Create(1)
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

func parseUnitStates(units []string) []string {
	states := make([]string, len(units))
	for i, unit := range units {
		cols := strings.SplitN(unit, "\t", 6)
		if len(cols) == 6 {
			states[i] = cols[2]
		}
	}
	return states
}

func activeCount(states []string) (count int) {
	for _, state := range states {
		if state == "active" {
			count++
		}
	}
	return
}

func fleetctl(args ...string) (string, string, error) {
	log.Printf("%s %s", fleetctlBinPath, strings.Join(args, " "))
	var stdoutBytes, stderrBytes bytes.Buffer
	cmd := exec.Command(fleetctlBinPath, args...)
	cmd.Stdout = &stdoutBytes
	cmd.Stderr = &stderrBytes
	err := cmd.Run()
	return stdoutBytes.String(), stderrBytes.String(), err
}

func fleetctlWithInput(input string, args ...string) (string, string, error) {
	log.Printf("%s %s", fleetctlBinPath, strings.Join(args, " "))
	var stdoutBytes, stderrBytes bytes.Buffer
	cmd := exec.Command(fleetctlBinPath, args...)
	cmd.Stdout = &stdoutBytes
	cmd.Stderr = &stderrBytes
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", "", err
	}

	if err = cmd.Start(); err != nil {
		return "", "", err
	}

	stdin.Write([]byte(input))
	stdin.Close()
	err = cmd.Wait()

	return stdoutBytes.String(), stderrBytes.String(), err
}

// Wait up to 10s to find the specified number of machines, retrying periodically.
func waitForNMachines(count int) ([]string, error) {
	var machines []string

	timeout := 10 * time.Second
	alarm := time.After(timeout)

	ticker := time.Tick(time.Second)
loop:
	for {
		select {
		case <-alarm:
			return machines, fmt.Errorf("Failed to find %d machines within %v", count, timeout)
		case <-ticker:
			stdout, _, err := fleetctl("list-machines", "--no-legend", "-l")
			stdout = strings.TrimSpace(stdout)
			if stdout == "" || err != nil {
				continue
			}

			machines = strings.Split(stdout, "\n")
			if len(machines) != count {
				continue
			}

			for k, v := range machines {
				machines[k] = strings.SplitN(v, "\t", 2)[0]
			}

			break loop
		}
	}

	return machines, nil
}

// Wait up to 10s to find the specified number of active units, retrying periodically.
func waitForNActiveUnits(count int) error {
	timeout := 10 * time.Second
	alarm := time.After(timeout)

	ticker := time.Tick(time.Second)
loop:
	for {
		select {
		case <-alarm:
			return fmt.Errorf("Failed to find %d active units within %v", count, timeout)
		case <-ticker:
			stdout, _, err := fleetctl("list-units", "--no-legend")
			stdout = strings.TrimSpace(stdout)
			if stdout == "" || err != nil {
				continue
			}

			units := strings.Split(stdout, "\n")
			states := parseUnitStates(units)
			if activeCount(states) != count {
				continue
			}

			break loop
		}
	}

	return nil
}
