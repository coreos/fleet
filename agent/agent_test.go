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

package agent

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/coreos/flt/job"
	"github.com/coreos/flt/machine"
	"github.com/coreos/flt/registry"
	"github.com/coreos/flt/unit"
)

func newTestUnitFromUnitContents(t *testing.T, name, contents string) *job.Unit {
	j := newTestJobFromUnitContents(t, name, contents)
	return &job.Unit{
		Name: j.Name,
		Unit: j.Unit,
	}
}

func newTestJobFromUnitContents(t *testing.T, name, contents string) *job.Job {
	u, err := unit.NewUnitFile(contents)
	if err != nil {
		t.Fatalf("error creating Unit from %q: %v", contents, err)
	}
	j := job.NewJob(name, *u)
	if j == nil {
		t.Fatalf("error creating Job %q from %q", name, u)
	}
	return j
}

func newNamedTestJobWithXFltValues(t *testing.T, name, metadata string) *job.Job {
	contents := fmt.Sprintf(`
[X-Flt]
%s
`, metadata)
	return newTestJobFromUnitContents(t, name, contents)
}

func newTestJobWithXFltValues(t *testing.T, metadata string) *job.Job {
	return newNamedTestJobWithXFltValues(t, "pong.service", metadata)
}

func TestAgentLoadUnloadUnit(t *testing.T) {
	uManager := unit.NewFakeUnitManager()
	usGenerator := unit.NewUnitStateGenerator(uManager)
	fReg := registry.NewFakeRegistry()
	mach := &machine.FakeMachine{MachineState: machine.MachineState{ID: "XXX"}}
	a := New(uManager, usGenerator, fReg, mach, time.Second)

	u := newTestUnitFromUnitContents(t, "foo.service", "")
	err := a.loadUnit(u)
	if err != nil {
		t.Fatalf("Failed calling Agent.loadUnit: %v", err)
	}

	units, err := a.units()
	if err != nil {
		t.Fatalf("Failed calling Agent.units: %v", err)
	}

	jsLoaded := job.JobStateLoaded
	expectUnits := unitStates{
		"foo.service": unitState{
			state: jsLoaded,
		},
	}

	if !reflect.DeepEqual(expectUnits, units) {
		t.Fatalf("Received unexpected collection of Units: %#v\nExpected: %#v", units, expectUnits)
	}

	a.unloadUnit("foo.service")

	units, err = a.units()
	if err != nil {
		t.Fatalf("Failed calling Agent.units: %v", err)
	}

	expectUnits = unitStates{}
	if !reflect.DeepEqual(expectUnits, units) {
		t.Fatalf("Received unexpected collection of Units: %#v\nExpected: %#v", units, expectUnits)
	}
}

func TestAgentLoadStartStopUnit(t *testing.T) {
	uManager := unit.NewFakeUnitManager()
	usGenerator := unit.NewUnitStateGenerator(uManager)
	fReg := registry.NewFakeRegistry()
	mach := &machine.FakeMachine{MachineState: machine.MachineState{ID: "XXX"}}
	a := New(uManager, usGenerator, fReg, mach, time.Second)

	u := newTestUnitFromUnitContents(t, "foo.service", "")

	err := a.loadUnit(u)
	if err != nil {
		t.Fatalf("Failed calling Agent.loadUnit: %v", err)
	}

	a.startUnit("foo.service")

	units, err := a.units()
	if err != nil {
		t.Fatalf("Failed calling Agent.units: %v", err)
	}

	jsLaunched := job.JobStateLaunched
	expectUnits := unitStates{
		"foo.service": unitState{
			state: jsLaunched,
		},
	}

	if !reflect.DeepEqual(expectUnits, units) {
		t.Fatalf("Received unexpected collection of Units: %#v\nExpected: %#v", units, expectUnits)
	}

	a.stopUnit("foo.service")

	units, err = a.units()
	if err != nil {
		t.Fatalf("Failed calling Agent.units: %v", err)
	}

	jsLoaded := job.JobStateLoaded
	expectUnits = unitStates{
		"foo.service": unitState{
			state: jsLoaded,
		},
	}

	if !reflect.DeepEqual(expectUnits, units) {
		t.Fatalf("Received unexpected collection of Units: %#v\nExpected: %#v", units, expectUnits)
	}
}
