package job

import (
	"testing"

	"github.com/coreos/fleet/machine"
)

func TestJobState(t *testing.T) {
	ms := &machine.MachineState{"XXX", "", make(map[string]string, 0)}
	js1 := NewJobState("loaded", "inactive", "dead", []string{}, ms)

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

	if js1.MachineState != ms {
		t.Fatal("job.JobState.MachineState does not match expected value")
	}
}

func TestJobStateNilMachineState(t *testing.T) {
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

	if js1.MachineState != nil {
		t.Fatal("job.JobState.MachineState != nil")
	}
}
