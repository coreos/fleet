package util

import (
	"bytes"
	"fmt"
	"io/ioutil"
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

type fleetfunc func(args ...string) (string, string, error)

func RunFleetctl(args ...string) (string, string, error) {
	log.Printf("%s %s", fleetctlBinPath, strings.Join(args, " "))
	var stdoutBytes, stderrBytes bytes.Buffer
	cmd := exec.Command(fleetctlBinPath, args...)
	cmd.Stdout = &stdoutBytes
	cmd.Stderr = &stderrBytes
	err := cmd.Run()
	return stdoutBytes.String(), stderrBytes.String(), err
}

func RunFleetctlWithInput(input string, args ...string) (string, string, error) {
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
func WaitForNMachines(fleetctl fleetfunc, count int) ([]string, error) {
	var machines []string
	timeout := 10 * time.Second
	alarm := time.After(timeout)

	ticker := time.Tick(250 * time.Millisecond)
loop:
	for {
		select {
		case <-alarm:
			return machines, fmt.Errorf("failed to find %d machines within %v", count, timeout)
		case <-ticker:
			stdout, _, err := fleetctl("list-machines", "--no-legend", "--full", "--fields", "machine")
			if err != nil {
				continue
			}

			stdout = strings.TrimSpace(stdout)

			found := 0
			if stdout != "" {
				machines = strings.Split(stdout, "\n")
				found = len(machines)
			}

			if found != count {
				continue
			}

			break loop
		}
	}

	return machines, nil
}

// WaitForNActiveUnits polls fleet for up to 10s, exiting when N units are
// found to be in an active state. It returns a map of active units to
// their target machines.
func WaitForNActiveUnits(fleetctl fleetfunc, count int) (map[string]UnitState, error) {
	var nactive int
	states := make(map[string]UnitState)

	timeout := 10 * time.Second
	alarm := time.After(timeout)

	ticker := time.Tick(250 * time.Millisecond)
loop:
	for {
		select {
		case <-alarm:
			return nil, fmt.Errorf("failed to find %d active units within %v (last found: %d)", count, timeout, nactive)
		case <-ticker:
			stdout, _, err := fleetctl("list-units", "--no-legend", "--full")
			stdout = strings.TrimSpace(stdout)
			if stdout == "" || err != nil {
				continue
			}

			lines := strings.Split(stdout, "\n")
			allStates := parseUnitStates(lines)
			active := filterActiveUnits(allStates)
			nactive = len(active)
			if nactive != count {
				continue
			}

			for _, state := range active {
				states[state.Name] = state
			}
			break loop
		}
	}

	return states, nil
}

type UnitState struct {
	Name        string
	JobState    string
	ActiveState string
	Machine     string
}

func parseUnitStates(units []string) map[string]UnitState {
	states := make(map[string]UnitState)
	for _, unit := range units {
		cols := strings.SplitN(unit, "\t", 7)
		if len(cols) == 7 {
			machine := strings.SplitN(cols[6], "/", 2)[0]
			states[cols[0]] = UnitState{cols[0], cols[2], cols[3], machine}
		}
	}
	return states
}

func filterActiveUnits(states map[string]UnitState) map[string]UnitState {
	filtered := make(map[string]UnitState)
	for unit, state := range states {
		if state.ActiveState == "active" {
			filtered[unit] = state
		}
	}
	return filtered
}

// tempUnit creates a local unit file with the given contents, returning
// the name of the file
func TempUnit(contents string) (string, error) {
	tmp, err := ioutil.TempFile(os.TempDir(), "fleet-test-unit-")
	if err != nil {
		return "", err
	}

	tmp.Write([]byte(contents))
	tmp.Close()

	svc := fmt.Sprintf("%s.service", tmp.Name())
	err = os.Rename(tmp.Name(), svc)
	if err != nil {
		os.Remove(tmp.Name())
		return "", err
	}

	return svc, nil
}
