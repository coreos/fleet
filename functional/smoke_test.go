package functional

import (
	"log"
	"bytes"
	"fmt"
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

func fleetctl(args ...string) (string, string, error) {
	log.Printf("%s %s", fleetctlBinPath, strings.Join(args, " "))
	var stdoutBytes, stderrBytes bytes.Buffer
	cmd := exec.Command(fleetctlBinPath, args...)
	cmd.Stdout = &stdoutBytes
	cmd.Stderr = &stderrBytes
	err := cmd.Run()
	return stdoutBytes.String(), stderrBytes.String(), err
}

func TestCluster(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatalf(err.Error())
	}
	defer cluster.DestroyAll()

	if err := cluster.Create(3); err != nil {
		t.Fatalf(err.Error())
	}

	var machines []string
	for i := 0; i <= 7; i++ {
		if i == 7 {
			t.Fatalf("Failed to find all three machines within the time limit")
		}

		log.Printf("Waiting 5s for fleet services to check in...")
		time.Sleep(5*time.Second)

		stdout, _, err := fleetctl("list-machines", "--no-legend", "-l")
		if err != nil {
			t.Fatalf("Failed to run list-machines: %v", err)
		}

		stdout = strings.TrimSpace(stdout)
		machines = strings.Split(stdout, "\n")
		if len(machines) == 3 {
			for k, v := range machines {
				machines[k] = strings.SplitN(v, "\t", 2)[0]
			}
			break
		}
	}

	for _, machine := range machines {
		if _, _, err := fleetctl("ssh", "--strict-host-key-checking=false", machine, "uptime"); err != nil {
			t.Errorf("Unable to SSH into fleet machine: %v", err)
		}
	}

	for i := 0; i < 5; i++ {
		unitName := fmt.Sprintf("fixtures/units/conflict.%d.service", i)
		_, _, err := fleetctl("start", "--no-block",  unitName)
		if err != nil {
			t.Fatalf("Failed to start unit %s: %v", unitName, err)
		}
	}

	stdout, _, err := fleetctl("list-units", "--no-legend")
	if err != nil {
		t.Fatalf("Failed to run list-units: %v", err)
	}

	units := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(units) != 5 {
		t.Fatalf("Did not find five units in cluster: \n%s", stdout)
	}

	for i := 0; i < 5; i++ {
		stdout, _, err := fleetctl("list-units", "--no-legend")
		if err != nil {
			t.Fatalf("Failed to run list-units: %v", err)
		}

		units := strings.Split(strings.TrimSpace(stdout), "\n")
		if len(units) != 5 {
			t.Fatalf("Did not find five units in cluster: \n%s", stdout)
		}

		states := parseUnitStates(units)
		if activeCount(states) != 3 {
			if i == 4 {
				t.Fatalf("Three units did not become active in time")
			}

			time.Sleep(time.Second)
			continue
		}

		break
	}

	if err := cluster.Create(2); err != nil {
		t.Fatalf(err.Error())
	}

	for i := 0; i <= 7; i++ {
		if i == 7 {
			t.Fatalf("Failed to find all three machines within the time limit")
		}

		log.Printf("Waiting 5s for fleet services to check in...")
		time.Sleep(5*time.Second)

		stdout, _, err := fleetctl("list-machines", "--no-legend", "-l")
		if err != nil {
			t.Fatalf("Failed to run list-machines: %v", err)
		}

		stdout = strings.TrimSpace(stdout)
		machines = strings.Split(stdout, "\n")
		if len(machines) == 5 {
			for k, v := range machines {
				machines[k] = strings.SplitN(v, "\t", 2)[0]
			}
			break
		}
	}

	for i := 0; i < 5; i++ {
		stdout, _, err := fleetctl("list-units", "--no-legend")
		if err != nil {
			t.Fatalf("Failed to run list-units: %v", err)
		}

		units := strings.Split(strings.TrimSpace(stdout), "\n")
		if len(units) != 5 {
			t.Fatalf("Did not find five units in cluster: \n%s", stdout)
		}

		states := parseUnitStates(units)
		if activeCount(states) != 5 {
			if i == 4 {
				t.Fatalf("Five units did not become active in time")
			}

			time.Sleep(time.Second)
			continue
		}

		break
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
