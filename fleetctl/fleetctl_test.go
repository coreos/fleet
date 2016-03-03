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

package main

import (
	"fmt"
	"testing"

	"github.com/coreos/fleet/client"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/schema"
	"github.com/coreos/fleet/unit"
	"github.com/coreos/fleet/version"

	"github.com/coreos/fleet/Godeps/_workspace/src/github.com/coreos/go-semver/semver"
)

type commandTestResults struct {
	description  string
	units        []string
	expectedExit int
}

func newFakeRegistryForCommands(unitPrefix string, unitCount int, template bool) client.API {
	// clear machineStates for every invocation
	machineStates = nil
	machines := []machine.MachineState{
		newMachineState("c31e44e1-f858-436e-933e-59c642517860", "1.2.3.4", map[string]string{"ping": "pong"}),
		newMachineState("595989bb-cbb7-49ce-8726-722d6e157b4e", "5.6.7.8", map[string]string{"foo": "bar"}),
	}

	jobs := make([]job.Job, 0)
	appendJobsForTests(&jobs, machines[0], unitPrefix, unitCount, template)
	appendJobsForTests(&jobs, machines[1], unitPrefix, unitCount, template)

	states := make([]unit.UnitState, 0)
	if template {
		state := unit.UnitState{
			UnitName:    fmt.Sprintf("%s@.service", unitPrefix),
			LoadState:   "loaded",
			ActiveState: "inactive",
			SubState:    "dead",
			MachineID:   machines[0].ID,
		}
		states = append(states, state)
		state.MachineID = machines[1].ID
		states = append(states, state)
	} else {
		for i := 1; i <= unitCount; i++ {
			state := unit.UnitState{
				UnitName:    fmt.Sprintf("%s%d.service", unitPrefix, i),
				LoadState:   "loaded",
				ActiveState: "active",
				SubState:    "listening",
				MachineID:   machines[0].ID,
			}
			states = append(states, state)
		}

		for i := 1; i <= unitCount; i++ {
			state := unit.UnitState{
				UnitName:    fmt.Sprintf("%s%d.service", unitPrefix, i),
				LoadState:   "loaded",
				ActiveState: "inactive",
				SubState:    "dead",
				MachineID:   machines[1].ID,
			}
			states = append(states, state)
		}
	}

	reg := registry.NewFakeRegistry()
	reg.SetMachines(machines)
	reg.SetUnitStates(states)
	reg.SetJobs(jobs)

	return &client.RegistryClient{Registry: reg}
}

func appendJobsForTests(jobs *[]job.Job, machine machine.MachineState, prefix string, unitCount int, template bool) {
	if template {
		// for start or load operations we may need to wait
		// during the creation of units, and since this is a
		// faked registry just set the 'Global' flag so we don't
		// block forever
		Options := []*schema.UnitOption{
			&schema.UnitOption{
				Section: "Unit",
				Name:    "Description",
				Value:   fmt.Sprintf("Template %s@.service", prefix),
			},
			&schema.UnitOption{
				Section: "X-Fleet",
				Name:    "Global",
				Value:   "true",
			},
		}
		uf := schema.MapSchemaUnitOptionsToUnitFile(Options)
		j := job.Job{
			Name:            fmt.Sprintf("%s@.service", prefix),
			Unit:            *uf,
			TargetMachineID: machine.ID,
		}
		*jobs = append(*jobs, j)
	} else {
		for i := 1; i <= unitCount; i++ {
			j := job.Job{
				Name:            fmt.Sprintf("%s%d.service", prefix, i),
				Unit:            unit.UnitFile{},
				TargetMachineID: machine.ID,
			}
			*jobs = append(*jobs, j)
		}
	}

	return
}

func newFakeRegistryForCheckVersion(v string) registry.ClusterRegistry {
	sv, err := semver.NewVersion(v)
	if err != nil {
		panic(err)
	}

	return registry.NewFakeClusterRegistry(sv, 0)
}

func TestCheckVersion(t *testing.T) {
	reg := newFakeRegistryForCheckVersion(version.Version)
	_, ok := checkVersion(reg)
	if !ok {
		t.Errorf("checkVersion failed but should have succeeded")
	}
	reg = newFakeRegistryForCheckVersion("9.0.0")
	msg, ok := checkVersion(reg)
	if ok || msg == "" {
		t.Errorf("checkVersion succeeded but should have failed")
	}
}

func TestMachineIDLegend(t *testing.T) {
	ms := machine.MachineState{
		ID:       "595989bb-cbb7-49ce-8726-722d6e157b4e",
		PublicIP: "5.6.7.8",
		Metadata: map[string]string{"foo": "bar"},
	}

	l := machineIDLegend(ms, true)
	if l != "595989bb-cbb7-49ce-8726-722d6e157b4e" {
		t.Errorf("Expected full machine ID, but it was %s\n", l)
	}

	l = machineIDLegend(ms, false)
	if l != "595989bb..." {
		t.Errorf("Expected partial machine ID, but it was %s\n", l)
	}
}

func TestFullLegendWithPublicIP(t *testing.T) {
	ms := machine.MachineState{
		ID:       "595989bb-cbb7-49ce-8726-722d6e157b4e",
		PublicIP: "5.6.7.8",
		Metadata: map[string]string{"foo": "bar"},
	}

	l := machineFullLegend(ms, false)
	if l != "595989bb.../5.6.7.8" {
		t.Errorf("Expected partial machine ID with public IP, but it was %s\n", l)
	}

	l = machineFullLegend(ms, true)
	if l != "595989bb-cbb7-49ce-8726-722d6e157b4e/5.6.7.8" {
		t.Errorf("Expected full machine ID with public IP, but it was %s\n", l)
	}
}

func TestFullLegendWithoutPublicIP(t *testing.T) {
	ms := machine.MachineState{
		ID:       "520983A8-FB9C-4A68-B49C-CED5BB2E9D08",
		Metadata: map[string]string{"foo": "bar"},
	}

	l := machineFullLegend(ms, false)
	if l != "520983A8..." {
		t.Errorf("Expected partial machine ID without public IP, but it was %s\n", l)
	}

	l = machineFullLegend(ms, true)
	if l != "520983A8-FB9C-4A68-B49C-CED5BB2E9D08" {
		t.Errorf("Expected full machine ID without public IP, but it was %s\n", l)
	}
}

var unitNameMangleTests = map[string]string{
	"foo":            "foo.service",
	"foo.1":          "foo.1.service",
	"foo/bar.socket": "bar.socket",
	"foo.socket":     "foo.socket",
	"foo.service":    "foo.service",
}

func TestUnitNameMangle(t *testing.T) {
	for n, w := range unitNameMangleTests {
		if g := unitNameMangle(n); g != w {
			t.Errorf("got %q, want %q", g, w)
		}
	}
}

func newUnitFile(t *testing.T, contents string) *unit.UnitFile {
	uf, err := unit.NewUnitFile(contents)
	if err != nil {
		t.Fatalf("error creating NewUnitFile from %s: %v", contents, err)
	}
	return uf
}

func TestCreateUnitFails(t *testing.T) {
	type fakeAPI struct {
		client.API
	}
	cAPI = fakeAPI{}
	var i int
	var un string
	var uf *unit.UnitFile
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("case %d: unexpectedly called API!", i)
			t.Logf("unit name: %q", un)
			t.Logf("unit file: %#v", uf)
		}
	}()
	type testCase struct {
		name string
		uf   *unit.UnitFile
	}
	var tt testCase
	testCases := []testCase{
		{
			"foo@{1,3}.service",
			newUnitFile(t, ``),
		},
		{
			"foo@{1..3}.service",
			newUnitFile(t, ``),
		},
		{
			"foo.{1-3}.service",
			newUnitFile(t, ``),
		},
		{
			"foo.service",
			nil,
		},
		{
			"foo.service",
			newUnitFile(t, `[X-Fleet]
	MachineOf=abcd
	Conflicts=abcd`),
		},
		{
			"foo.service",
			newUnitFile(t, `[X-Fleet]
MachineOf=abcd
Conflicts=abcd`),
		},
		{
			"foo.service",
			newUnitFile(t, `[X-Fleet]
Global=true
MachineOf=abcd`),
		},
		{
			"foo.service",
			newUnitFile(t, `[X-Fleet]
Global=true
MachineOf=zxcvq`),
		},
		{
			"foo.service",
			newUnitFile(t, `[X-Fleet]
Global=true
Conflicts=bar`),
		},
	}
	for i, tt = range testCases {
		un = tt.name
		uf = tt.uf
		if _, err := createUnit(un, uf); err == nil {
			t.Errorf("case %d did not return error as expected!", i)
			t.Logf("unit name: %v", un)
			t.Logf("unit file: %#v", uf)
		}
	}
}
