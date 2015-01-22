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

package engine

import (
	"testing"
	"time"

	"github.com/coreos/fleet/registry"
)

func TestEnsureEngineVersionMatch(t *testing.T) {
	tests := []struct {
		current int
		target  int

		wantMatch   bool
		wantVersion int
	}{
		{
			current:     0,
			target:      1,
			wantMatch:   true,
			wantVersion: 1,
		},
		{
			current:     1,
			target:      1,
			wantMatch:   true,
			wantVersion: 1,
		},

		{
			current:     2,
			target:      1,
			wantMatch:   false,
			wantVersion: 2,
		},
	}

	for i, tt := range tests {
		cReg := registry.NewFakeClusterRegistry(nil, tt.current)
		gotMatch := ensureEngineVersionMatch(cReg, tt.target)
		if tt.wantMatch != gotMatch {
			t.Errorf("case %d: ensureEngineVersionMatch result incorrect: want=%t got=%t", i, tt.wantMatch, gotMatch)
		}

		gotVersion, _ := cReg.EngineVersion()
		if tt.wantVersion != gotVersion {
			t.Errorf("case %d: resulting envine version incorrect: want=%d got=%d", i, tt.wantVersion, gotVersion)
		}
	}
}

type leaseMeta struct {
	machID string
	ver    int
}

func TestAcquireLeadership(t *testing.T) {
	tests := []struct {
		exist       *leaseMeta
		local       leaseMeta
		wantAcquire bool
	}{
		// able to acquire if lease does not already exist
		{
			exist:       nil,
			local:       leaseMeta{machID: "XXX", ver: 12},
			wantAcquire: true,
		},

		// steal if lease exists at lower version
		{
			exist:       &leaseMeta{machID: "YYY", ver: 0},
			local:       leaseMeta{machID: "XXX", ver: 1},
			wantAcquire: true,
		},

		// unable to acquire if lease exists at higher version
		{
			exist:       &leaseMeta{machID: "YYY", ver: 10},
			local:       leaseMeta{machID: "XXX", ver: 2},
			wantAcquire: false,
		},

		// unable to acquire if lease exists at same version
		{
			exist:       &leaseMeta{machID: "YYY", ver: 2},
			local:       leaseMeta{machID: "XXX", ver: 2},
			wantAcquire: false,
		},
	}

	for i, tt := range tests {
		lReg := registry.NewFakeLeaseRegistry()

		if tt.exist != nil {
			lReg.SetLease(engineLeaseName, tt.exist.machID, tt.exist.ver, time.Millisecond)
		}

		got := acquireLeadership(lReg, tt.local.machID, tt.local.ver, time.Millisecond)

		if tt.wantAcquire != (isLeader(got, tt.local.machID)) {
			t.Errorf("case %d: wantAcquire=%t but got %#v", i, tt.wantAcquire, got)
		}
	}
}

func TestIsLeader(t *testing.T) {
	tests := []struct {
		lease      *leaseMeta
		machID     string
		wantLeader bool
	}{
		// not a leader if lease is null
		{
			lease:      nil,
			machID:     "XXX",
			wantLeader: false,
		},

		// not a leader if lease is held by a different machines
		{
			lease:      &leaseMeta{machID: "YYY"},
			machID:     "XXX",
			wantLeader: false,
		},

		// leader if lease is valid and held by current machine
		{
			lease:      &leaseMeta{machID: "XXX"},
			machID:     "XXX",
			wantLeader: true,
		},
	}

	for i, tt := range tests {
		lReg := registry.NewFakeLeaseRegistry()

		if tt.lease != nil {
			lReg.SetLease(engineLeaseName, tt.lease.machID, tt.lease.ver, time.Millisecond)
		}

		lease, _ := lReg.GetLease(engineLeaseName)

		got := isLeader(lease, tt.machID)
		if tt.wantLeader != got {
			t.Errorf("case %d: wantLeader=%t but got %t", i, tt.wantLeader, got)
		}
	}
}
