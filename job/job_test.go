package job

import (
	"testing"

	"github.com/coreos/coreinit/machine"
)


func TestJobNilStateNilPayload(t *testing.T) {
	j1 := NewJob("ping", nil, nil)
	j2 := Job{"ping", nil, nil}

	if *j1 != j2 {
		t.Error("job.NewJob factory failed to produce appropriate job.Job")
	}

	if j1.Name != "ping" {
		t.Fatal("job.Job.Name != 'ping'")
	}

	if j1.State != nil {
		t.Fatal("job.Job.State != nil")
	}

	if j1.Payload != nil {
		t.Fatal("job.Job.Payload != nil")
	}
}

func TestJob(t *testing.T) {
	mach := machine.New("XXX")
	js1 := NewJobState("inactive", mach)
	jp1 := NewJobPayload("systemd-service", "echo")

	j1 := NewJob("pong", js1, jp1)
	j2 := Job{"pong", js1, jp1}

	if *j1 != j2 {
		t.Error("job.NewJob factory failed to produce appropriate job.Job")
	}

	if j1.Name != "pong" {
		t.Fatal("job.Job.Name != 'pong'")
	}

	if j1.State != js1 {
		t.Fatal("job.Job.State does not match expected value")
	}

	if j1.Payload != jp1 {
		t.Fatal("job.Job.Payload does not match expected value")
	}

}

func TestJobState(t *testing.T) {
	mach := machine.New("XXX")
	js1 := NewJobState("inactive", mach)
	js2 := JobState{"inactive", mach}

	if *js1 != js2 {
		t.Error("job.NewJobState factory failed to produce appropriate job.JobState")
	}

	if js1.State != "inactive" {
		t.Fatal("job.JobState.State != 'inactive'")
	}

	if js1.Machine != mach {
		t.Fatal("job.JobState.Machine does not match expected value")
	}
}

func TestJobStateNilMachine(t *testing.T) {
	js1 := NewJobState("active", nil)
	js2 := JobState{"active", nil}

	if *js1 != js2 {
		t.Error("job.NewJobState factory failed to produce appropriate job.JobState")
	}

	if js1.State != "active" {
		t.Fatal("job.JobState.State != 'active'")
	}

	if js1.Machine != nil {
		t.Fatal("job.JobState.Machine != nil")
	}
}

func TestJobPayload(t *testing.T) {
	jp1 := NewJobPayload("systemd-service", "echo")
	jp2 := JobPayload{"systemd-service", "echo"}

	if *jp1 != jp2 {
		t.Error("job.NewJobPayload factory failed to produce appropriate job.JobPayload")
	}

	if jp1.Type != "systemd-service" {
		t.Fatal("job.JobPayload.Type != 'systemd-service'")
	}

	if jp1.Value != "echo" {
		t.Fatal("job.JobPayload.Value != 'echo'")
	}
}
