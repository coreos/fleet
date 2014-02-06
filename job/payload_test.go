package job

import (
	"testing"

	"github.com/coreos/fleet/unit"
)

func TestNewJobPayloadBadType(t *testing.T) {
	j := NewJobPayload("foo.unknown", *unit.NewSystemdUnitFile("echo"))
	_, err := j.Type()

	if err == nil {
		t.Fatal("Expected non-nil error")
	}
}

func TestNewJobPayload(t *testing.T) {
	payload := NewJobPayload("echo.service", *unit.NewSystemdUnitFile("Echo"))

	if payload.Name != "echo.service" {
		t.Errorf("Payload has unexpected name '%s'", payload.Name)
	}

	if pt, _ := payload.Type(); pt != "systemd-service" {
		t.Errorf("Payload has unexpected Type '%s'", pt)
	}
}
