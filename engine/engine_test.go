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

		if tt.wantAcquire != (got != nil) {
			t.Errorf("case %d: wantAcquire=%t but got %#v", i, tt.wantAcquire, got)
		}
	}
}
