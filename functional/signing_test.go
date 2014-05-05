package functional

import (
	"testing"

	"github.com/coreos/fleet/functional/platform"
)

func TestSignedRequests(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.Destroy()

	cfg := platform.MachineConfig{VerifyUnits: true}
	if err := cluster.CreateMember("1", cfg); err != nil {
		t.Fatal(err)
	}
	_, err = cluster.WaitForNMachines(1)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = cluster.Fleetctl("submit", "--sign=false", "fixtures/units/hello.service")
	if err != nil {
		t.Fatalf("Failed submitting hello.service: %v", err)
	}

	// The start command should succeed, but the unit should not actually get scheduled
	// and started on an agent since it is not signed.
	_, _, err = cluster.Fleetctl("load", "--no-block", "fixtures/units/hello.service")
	if err != nil {
		t.Fatalf("Failed calling load on hello.service: %v", err)
	}

	_, _, err = cluster.Fleetctl("start", "--no-block", "--sign=true", "fixtures/units/goodbye.service")
	if err != nil {
		t.Fatalf("Failed starting goodbye.service: %v", err)
	}

	units, err := cluster.WaitForNActiveUnits(1)
	if err != nil {
		t.Fatal(err)
	}

	_, ok := units["goodbye.service"]
	if len(units) != 1 || !ok {
		t.Fatalf("Expected goodbye.service to be sole active unit, got %v", units)
	}
}
