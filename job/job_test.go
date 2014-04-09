package job

import (
	"testing"

	"github.com/coreos/fleet/unit"
)

func TestNewJobNilStateNilPayload(t *testing.T) {
	j1 := NewJob("ping.service", nil)

	if j1.Name != "ping.service" {
		t.Fatal("job.Job.Name != 'ping.service'")
	}

	if j1.State != nil {
		t.Fatal("job.Job.State != nil")
	}

	if j1.Payload != nil {
		t.Fatal("job.Job.Payload != nil")
	}
}

func TestNewJob(t *testing.T) {
	jp1 := NewJobPayload("echo.service", *unit.NewSystemdUnitFile("Echo"))
	j1 := NewJob("pong.service", jp1)

	if j1.Name != "pong.service" {
		t.Fatal("job.Job.Name != 'pong.service'")
	}

	if j1.Payload.Name != jp1.Name {
		t.Fatal("job.Job.Payload does not match expected value")
	}
}
