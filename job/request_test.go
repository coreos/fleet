package job

import (
	"testing"

	"github.com/coreos/coreinit/unit"
)

func TestNewJobRequestNilPayloads(t *testing.T) {
	_, err := NewJobRequest(nil)
	if err == nil {
		t.Error("Expected error when using nil payloads")
	}
}

func TestNewJobRequestEmptyPayloads(t *testing.T) {
	payloads := []JobPayload{}
	_, err := NewJobRequest(payloads)
	if err == nil {
		t.Error("Expected error when using empty payloads slice")
	}
}

func TestNewJobRequest(t *testing.T) {
	payload := NewJobPayload("pong.service", *unit.NewSystemdUnitFile("pong"))
	payloads := []JobPayload{*payload}
	jr, err := NewJobRequest(payloads)
	if err != nil {
		t.Errorf("Not expecting error:", err)
	}

	if len(jr.Payloads) != 1 || jr.Payloads[0].Name != payloads[0].Name {
		t.Error("Payloads does not match expected value")
	}

	if len(jr.ID.String()) != 36 {
		t.Errorf("ID appears invalid: %s", jr.ID.String())
	}
}
