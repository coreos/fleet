package job

import (
	"testing"
)

func TestNewJobRequestNilPayloads(t *testing.T) {
	_, err := NewJobRequest(nil, nil)
	if err == nil {
		t.Error("Expected error when using nil payloads")
	}
}

func TestNewJobRequestEmptyPayloads(t *testing.T) {
	payloads := []JobPayload{}
	_, err := NewJobRequest(payloads, nil)
	if err == nil {
		t.Error("Expected error when using empty payloads slice")
	}
}

func TestNewJobRequestNilMachines(t *testing.T) {
	payloads := []JobPayload{JobPayload{"pong.service", "pong"}}
	jr, err := NewJobRequest(payloads, nil)
	if err != nil {
		t.Errorf("Not expecting error:", err)
	}

	if len(jr.Payloads) != 1 || jr.Payloads[0] != payloads[0] {
		t.Error("Payloads does not match expected value")
	}
}

func TestNewJobRequestIDGeneration(t *testing.T) {
	payloads := []JobPayload{JobPayload{"pong.service", "pong"}}
	jr, err := NewJobRequest(payloads, nil)
	if err != nil {
		t.Errorf("Not expecting error:", err)
	}

	if len(jr.ID.String()) != 36 {
		t.Errorf("ID appears invalid: %s", jr.ID.String())
	}
}
