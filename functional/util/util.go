// Copyright 2014 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
)

var fltctlBinPath string

func init() {
	fltctlBinPath = os.Getenv("FLEETCTL_BIN")
	if fltctlBinPath == "" {
		fmt.Println("FLEETCTL_BIN environment variable must be set")
		os.Exit(1)
	} else if _, err := os.Stat(fltctlBinPath); err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}

	if os.Getenv("SSH_AUTH_SOCK") == "" {
		fmt.Println("SSH_AUTH_SOCK environment variable must be set")
		os.Exit(1)
	}
}

type fltfunc func(args ...string) (string, string, error)

func RunFltctl(args ...string) (string, string, error) {
	log.Printf("%s %s", fltctlBinPath, strings.Join(args, " "))
	var stdoutBytes, stderrBytes bytes.Buffer
	cmd := exec.Command(fltctlBinPath, args...)
	cmd.Stdout = &stdoutBytes
	cmd.Stderr = &stderrBytes
	err := cmd.Run()
	return stdoutBytes.String(), stderrBytes.String(), err
}

func RunFltctlWithInput(input string, args ...string) (string, string, error) {
	log.Printf("%s %s", fltctlBinPath, strings.Join(args, " "))
	var stdoutBytes, stderrBytes bytes.Buffer
	cmd := exec.Command(fltctlBinPath, args...)
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
	IP          string
}

func ParseUnitStates(units []string) (states []UnitState) {
	for _, unit := range units {
		cols := strings.SplitN(unit, "\t", 3)
		if len(cols) == 3 {
			mParts := strings.SplitN(cols[2], "/", 2)
			states = append(states, UnitState{cols[0], cols[1], mParts[0], mParts[1]})
		}
	}
	return states
}

func FilterActiveUnits(states []UnitState) (filtered []UnitState) {
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
	tmp, err := ioutil.TempFile(os.TempDir(), "flt-test-unit-")
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
