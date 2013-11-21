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
	js1 := NewJobState("inactive", mach)
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
