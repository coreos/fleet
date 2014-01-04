package job

import (
	"testing"
)

func TestNewJobPayloadBadType(t *testing.T) {
	j, err := NewJobPayload("foo.unknown", "echo", make(map[string][]string, 0))

	if err == nil {
		t.Fatal("Expected non-nil error")
	}

	if j != nil {
		t.Fatal("Expected nil response")
	}
}

func TestNewJobPayload(t *testing.T) {
	payload, _ := NewJobPayload("echo.service", "Echo", map[string][]string{})
	payloads := []JobPayload{*payload}
	jr, err := NewJobRequest(payloads)
	if err != nil {
		t.Errorf("Not expecting error:", err)
	}

	if len(jr.Payloads) != 1 || jr.Payloads[0].Name != payloads[0].Name {
		t.Error("Payloads does not match expected value")
	}
}
