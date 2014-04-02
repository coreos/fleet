package functional

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
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
func waitForNActiveUnits(count int) ([]string, error) {
	var units []string

	timeout := 10 * time.Second
	alarm := time.After(timeout)

	ticker := time.Tick(time.Second)
loop:
	for {
		select {
		case <-alarm:
			return nil, fmt.Errorf("Failed to find %d active units within %v", count, timeout)
		case <-ticker:
			stdout, _, err := fleetctl("list-units", "--no-legend")
			stdout = strings.TrimSpace(stdout)
			if stdout == "" || err != nil {
				continue
			}

			lines := strings.Split(stdout, "\n")
			states := parseUnitStates(lines)
			active := filterActiveUnits(states)
			if len(active) != count {
				continue
			}

			for unit, _ := range active {
				units = append(units, unit)
			}
			break loop
		}
	}

	return units, nil
}

func parseUnitStates(units []string) map[string]string {
	states := make(map[string]string)
	for _, unit := range units {
		cols := strings.SplitN(unit, "\t", 6)
		if len(cols) == 6 {
			states[cols[0]] = cols[2]
		}
	}
	return states
}

func filterActiveUnits(states map[string]string) map[string]string {
	filtered := make(map[string]string)
	for unit, state := range states {
		if state == "active" {
			filtered[unit] = state
		}
	}
	return filtered
}
