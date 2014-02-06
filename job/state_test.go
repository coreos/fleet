package job

import (
	"testing"

	"github.com/coreos/fleet/machine"
)

func TestJobState(t *testing.T) {
	mach := machine.New("XXX", "", make(map[string]string, 0))
	js1 := NewJobState("loaded", "inactive", "dead", []string{}, mach)

	if js1.LoadState != "loaded" {
		t.Fatal("job.JobState.LoadState != 'loaded'")
	}

	if js1.ActiveState != "inactive" {
		t.Fatal("job.JobState.ActiveState != 'inactive'")
	}

	if js1.SubState != "dead" {
		t.Fatal("job.JobState.SubState != 'dead'")
	}

	if len(js1.Sockets) != 0 {
		t.Fatal("job.JobState.Sockets does not match expected length")
	}

	if js1.Machine != mach {
		t.Fatal("job.JobState.Machine does not match expected value")
	}
}

func TestJobStateNilMachine(t *testing.T) {
	js1 := NewJobState("loaded", "active", "listening", []string{}, nil)

	if js1.LoadState != "loaded" {
		t.Fatal("job.JobState.LoadState != 'loaded'")
	}

	if js1.ActiveState != "active" {
		t.Fatal("job.JobState.ActiveState != 'active'")
	}

	if js1.SubState != "listening" {
		t.Fatal("job.JobState.SubState != 'listening'")
	}

	if len(js1.Sockets) != 0 {
		t.Fatal("job.JobState.Sockets does not match expected length")
	}

	if js1.Machine != nil {
		t.Fatal("job.JobState.Machine != nil")
	}
}
