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

// WaitForNActiveUnits polls fleet for up to 15s, exiting when N units are
// found to be in an active state. It returns a map of unit name to a list of
// active UnitStates for that unit
func WaitForNActiveUnits(fleetctl fleetfunc, count int) (map[string][]UnitState, error) {
	var nactive int
	states := make(map[string][]UnitState)

	timeout := 15 * time.Second
	alarm := time.After(timeout)

	ticker := time.Tick(250 * time.Millisecond)
loop:
	for {
		select {
		case <-alarm:
			return nil, fmt.Errorf("failed to find %d active units within %v (last found: %d)", count, timeout, nactive)
		case <-ticker:
			stdout, _, err := fleetctl("list-units", "--no-legend", "--full", "--fields", "unit,active,machine")
			stdout = strings.TrimSpace(stdout)
			if err != nil {
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
				name := state.Name
				if _, ok := states[name]; !ok {
					states[name] = []UnitState{}
				}
				states[name] = append(states[name], state)
			}
			break loop
		}
	}

	return states, nil
}

// ActiveToSingleStates takes a map of active states (such as that returned by
// WaitForNActiveUnits) and ensures that each unit has at most a single active
// state. It returns a mapping of unit name to a single UnitState.
func ActiveToSingleStates(active map[string][]UnitState) (map[string]UnitState, error) {
	states := make(map[string]UnitState)
	for name, us := range active {
		if len(us) != 1 {
			return nil, fmt.Errorf("unit %s running in multiple locations: %v", name, us)
		}
		states[name] = us[0]
	}
	return states, nil
}

type UnitState struct {
	Name        string
	ActiveState string
	Machine     string
}

func parseUnitStates(units []string) (states []UnitState) {
	for _, unit := range units {
		cols := strings.SplitN(unit, "\t", 3)
		if len(cols) == 3 {
			machine := strings.SplitN(cols[2], "/", 2)[0]
			states = append(states, UnitState{cols[0], cols[1], machine})
		}
	}
	return states
}

func filterActiveUnits(states []UnitState) (filtered []UnitState) {
	for _, state := range states {
		if state.ActiveState == "active" {
			filtered = append(filtered, state)
		}
	}
	return
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
