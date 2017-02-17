// Copyright 2016 The fleet Authors
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

// +build dummytest !all

package functional

import (
	"math"
	"testing"
	"time"

	"github.com/coreos/fleet/functional/platform"
)

// TestDummy sets up a functional test environment, but does nothing,
// just waits forever.
func TestDummy(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.Destroy(t)

	m, err := cluster.CreateMember()
	if err != nil {
		t.Fatal(err)
	}

	_, err = cluster.WaitForNMachines(m, 1)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(math.MaxInt32 * time.Second)
}
