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

package functional

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/systemd"
	"github.com/coreos/fleet/unit"
)

func TestSystemdUnitFlow(t *testing.T) {
	uDir, err := ioutil.TempDir("", "fleet-")
	if err != nil {
		t.Fatalf("Failed creating tempdir: %v", err)
	}
	defer os.RemoveAll(uDir)

	mgr, err := systemd.NewSystemdUnitManager(uDir, false)
	if err != nil {
		t.Fatalf("Failed initializing SystemdUnitManager: %v", err)
	}

	units, err := mgr.Units()
	if err != nil {
		t.Fatalf("Failed calling Units(): %v", err)
	}

	if len(units) > 0 {
		t.Fatalf("Expected no units to be returned, got %v", units)
	}

	contents := `[Service]
ExecStart=/usr/bin/sleep 3000
`
	name := fmt.Sprintf("fleet-unit-%d.service", rand.Int63())
	uf, err := unit.NewUnitFile(contents)
	if err != nil {
		t.Fatalf("Invalid unit file: %v", err)
	}
	hash := uf.Hash().String()
	j := job.NewJob(name, *uf)

	if err := mgr.Load(j.Name, j.Unit); err != nil {
		t.Fatalf("Failed loading job: %v", err)
	}

	units, err = mgr.Units()
	if err != nil {
		t.Fatalf("Failed calling Units(): %v", err)
	}

	if !reflect.DeepEqual([]string{name}, units) {
		t.Fatalf("Expected [hello.service], got %v", units)
	}

	err = waitForUnitState(mgr, name, unit.UnitState{"loaded", "inactive", "dead", "", hash, ""})
	if err != nil {
		t.Error(err.Error())
	}

	mgr.TriggerStart(name)

	err = waitForUnitState(mgr, name, unit.UnitState{"loaded", "active", "running", "", hash, ""})
	if err != nil {
		t.Error(err.Error())
	}

	mgr.TriggerStop(name)

	mgr.Unload(name)

	units, err = mgr.Units()
	if err != nil {
		t.Fatalf("Failed calling Units(): %v", err)
	}

	if len(units) > 0 {
		t.Fatalf("Expected no units to be returned, got %v", units)
	}
}

func waitForUnitState(mgr unit.UnitManager, name string, want unit.UnitState) error {
	timeout := time.After(time.Second)
	for {
		select {
		case <-timeout:
			return fmt.Errorf("Timed out waiting for state of %s to match %#v", name, want)
		default:
		}

		got, err := mgr.GetUnitState(name)
		if err != nil {
			return err
		}

		if reflect.DeepEqual(want, *got) {
			return nil
		}
	}
}
