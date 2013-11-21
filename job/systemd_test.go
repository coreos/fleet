package job

import (
	"testing"
)

func TestNewJobPayloadFromSystemdUnitService(t *testing.T) {
	jp1, _ := NewJobPayloadFromSystemdUnit("echo.service", "echo")
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

func TestNewJobPayloadFromSystemdUnitSocket(t *testing.T) {
	jp1, _ := NewJobPayloadFromSystemdUnit("echo.socket", "echo")
	jp2 := JobPayload{"systemd-socket", "echo"}

	if *jp1 != jp2 {
		t.Error("job.NewJobPayload factory failed to produce appropriate job.JobPayload")
	}

	if jp1.Type != "systemd-socket" {
		t.Fatal("job.JobPayload.Type != 'systemd-socket'")
	}

	if jp1.Value != "echo" {
		t.Fatal("job.JobPayload.Value != 'echo'")
	}
}

func TestNewJobPayloadFromSystemdUnitUnknown(t *testing.T) {
	jp, err := NewJobPayloadFromSystemdUnit("echo.target", "echo")

	if err == nil {
		t.Fatal("Expected non-nil error")
	}

	if jp != nil {
		t.Fatal("Expected nil *JobPayload")
	}
}
