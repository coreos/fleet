package version

import (
	"testing"
	"time"

	"github.com/coreos/fleet/event"
	"github.com/coreos/fleet/machine"
)

func TestNegotiatorInitialization(t *testing.T) {
	node1, err := NewNegotiator("n1", 3, 7)

	if err != nil {
		t.Fatalf("NewNegotiator returned error: %v", err)
	}

	if node1.CurrentVersion != -1 {
		t.Errorf("Node should have returned -1, received %d", node1.CurrentVersion)
	}

	if node1.Name != "n1" {
		t.Errorf("Node mis-reported its name: %s", node1.Name)
	}

	if node1.MinVersion != 3 {
		t.Errorf("Node mis-reported its minimum version: %d", node1.MinVersion)
	}

	if node1.MaxVersion != 7 {
		t.Errorf("Node mis-reported its maximum version: %d", node1.MaxVersion)
	}
}

func TestNegotiatorSetCurrentVersionSuccessful(t *testing.T) {
	node1, _ := NewNegotiator("n1", 3, 7)
	node1.SetCurrentVersion(4)

	if node1.CurrentVersion != 4 {
		t.Fatalf("Node should have returned 4, received %d", node1.CurrentVersion)
	}
}

func TestNegotiatorSetCurrentVersionUnrecognized(t *testing.T) {
	node1, _ := NewNegotiator("n1", 3, 7)
	node1.SetCurrentVersion(17)

	if node1.CurrentVersion != 17 {
		t.Errorf("Node should have returned 17, received %d", node1.CurrentVersion)
	}
}

func TestNegotiatorInitializationInvertedVersions(t *testing.T) {
	_, err := NewNegotiator("n1", 7, 3)
	if err == nil {
		t.Fatalf("Expected error was not raised")
	}
}

func TestNegotiatorInitializationNegativeMinVersion(t *testing.T) {
	_, err := NewNegotiator("n1", -2, 3)
	if err == nil {
		t.Fatalf("Expected error was not raised")
	}
}

func TestMaxVersionPossible(t *testing.T) {
	node1, _ := NewNegotiator("n1", 3, 5)
    node2, _ := NewNegotiator("n2", 0, 6)
	node3, _ := NewNegotiator("n3", 7, 10)
	node4, _ := NewNegotiator("n4", 2, 9)
	negotiators := []Negotiator{*node1, *node2, *node3, *node4}
	max, _ := MaxVersionPossible(negotiators)
	if max != 5 {
		t.Fatalf("Failed to determine max version is 5, returned %d", max)
	}
}

func TestMaxVersionPossibleNoNegotiators(t *testing.T) {
	negotiators := []Negotiator{}
	_, err := MaxVersionPossible(negotiators)
	if err == nil {
		t.Fatalf("Failed to return expected error")
	}
}

func TestClusterSingleNodeUpgrade(t *testing.T) {
	eb := event.NewEventBus()
	eb.Listen()
	defer eb.Stop()

	cluster := newStubClusterState(eb)
	node1, _ := NewNegotiator("n1", 3, 4)

	eh := eventHandler{node1, cluster}
	mach1 := machine.New("n1", "", make(map[string]string, 0))
	eb.AddListener("n1", mach1, &eh)

	err := cluster.Publish(node1, time.Second)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	timeout := time.After(time.Second)
	for {
		select {
		case <-timeout:
			t.Fatalf("Failed to upgrade cluster within 1s")
		default:
			if version, _, _ := cluster.Version(); version == 4 { return }
			time.Sleep(time.Millisecond*50)
		}
	}
}

func TestClusterNewerNodeDoesNotForceUpgrade(t *testing.T) {
	eb := event.NewEventBus()
	eb.Listen()
	defer eb.Stop()

	cluster := newStubClusterState(eb)
	node1, _ := NewNegotiator("n1", 3, 4)
	node2, _ := NewNegotiator("n2", 3, 5)

	eh1 := eventHandler{node1, cluster}
	mach1 := machine.New("n1", "", make(map[string]string, 0))
	eb.AddListener("n1", mach1, &eh1)

	eh2 := eventHandler{node2, cluster}
	mach2 := machine.New("n2", "", make(map[string]string, 0))
	eb.AddListener("n2", mach2, &eh2)

	cluster.Publish(node1, time.Second)
	cluster.Publish(node2, time.Second)

	time.Sleep(time.Second)

	if version, _, _ := cluster.Version(); version != 4 {
		t.Fatalf("Final cluster version was %d, not 4", version)
	}
}
