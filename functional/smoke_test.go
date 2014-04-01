package functional

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
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

func waitForNMachines(count int) ([]string, error) {
	var machines []string
	for i := 0; i <= 7; i++ {
		if i == 7 {
			return nil, fmt.Errorf("Failed to find %d machines within the time limit", count)
		}

		log.Printf("Waiting 5s for %d fleet services to check in...", count)
		time.Sleep(5 * time.Second)

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

		break
	}

	return machines, nil
}

func waitForNActiveUnits(count int) error {
	for i := 0; i <= 6; i++ {
		if i == 6 {
			return fmt.Errorf("Failed to find %d active units within the time limit", count)
		}

		log.Printf("Waiting 1s for %d fleet units to become active...", count)
		time.Sleep(time.Second)

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

		break
	}

	return nil
}
