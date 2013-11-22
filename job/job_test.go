package job

import (
	"testing"

	"github.com/coreos/coreinit/machine"
)


func TestNewJobNilStateNilPayload(t *testing.T) {
	j1, _ := NewJob("ping.service", nil, nil)
	j2 := Job{"ping.service", "systemd-service", nil, nil}

	if *j1 != j2 {
		t.Error("job.NewJob factory failed to produce appropriate job.Job")
	}

	if j1.Name != "ping.service" {
		t.Fatal("job.Job.Name != 'ping.service'")
	}

	if j1.Type != "systemd-service" {
		t.Fatal("job.Job.Name != 'systemd-service'")
	}

	if j1.State != nil {
		t.Fatal("job.Job.State != nil")
	}

	if j1.Payload != nil {
		t.Fatal("job.Job.Payload != nil")
	}
}

func TestNewJob(t *testing.T) {
	mach := machine.New("XXX")
	js1 := NewJobState("loaded", "inactive", "running", []string{}, mach)
	jp1 := &JobPayload{"echo"}

	j1, _ := NewJob("pong.service", js1, jp1)
	j2 := Job{"pong.service", "systemd-service", js1, jp1}

	if *j1 != j2 {
		t.Error("job.NewJob factory failed to produce appropriate job.Job")
	}

	if j1.Name != "pong.service" {
		t.Fatal("job.Job.Name != 'pong.service'")
	}

	if j1.Type != "systemd-service" {
		t.Fatal("job.Job.Name != 'systemd-service'")
	}

	if j1.State != js1 {
		t.Fatal("job.Job.State does not match expected value")
	}

	if j1.Payload != jp1 {
		t.Fatal("job.Job.Payload does not match expected value")
	}

}

func TestNewJobBadType(t *testing.T) {
	j, err := NewJob("bad-type", nil, nil)

	if err == nil {
		t.Fatal("Expected non-nil error")
	}

	if j != nil {
		t.Fatal("Expected nil response")
	}
}

func TestJobState(t *testing.T) {
	mach := machine.New("XXX")
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
