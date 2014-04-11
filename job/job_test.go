package job

import (
	"testing"

	"github.com/coreos/fleet/unit"
)

func TestNewJob(t *testing.T) {
	jp1 := NewJobPayload("echo.service", *unit.NewSystemdUnitFile("Echo"))
	j1 := NewJob("pong.service", *jp1)

	if j1.Name != "pong.service" {
		t.Error("job.Job.Name != 'pong.service'")
	}

	if j1.Payload.Name != "echo.service" {
		t.Error("job.Job.Payload.Name != 'echo.service'")
	}
}
