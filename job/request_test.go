package job

import (
	"testing"
)

func TestNewJobRequestNilPayloadsNoRequirements(t *testing.T) {
	_, err := NewJobRequest(nil, make([]string, 0), make(map[string][]string, 0))
	if err == nil {
		t.Error("Expected error when using nil payloads")
	}
}

func TestNewJobRequestEmptyPayloadsNoRequirements(t *testing.T) {
	payloads := []JobPayload{}
	_, err := NewJobRequest(payloads, make([]string, 0), make(map[string][]string, 0))
	if err == nil {
		t.Error("Expected error when using empty payloads slice")
	}
}

func TestNewJobRequest(t *testing.T) {
	payloads := []JobPayload{JobPayload{"pong.service", "pong"}}
	jr, err := NewJobRequest(payloads, []string{"ping.service"}, map[string][]string{"foo": []string{"bar"}})
	if err != nil {
		t.Errorf("Not expecting error:", err)
	}

	if len(jr.Payloads) != 1 || jr.Payloads[0] != payloads[0] {
		t.Error("Payloads does not match expected value")
	}

	if len(jr.Peers) != 1 || jr.Peers[0] != "ping.service" {
		t.Fatal("JobRequest.Peers are incorrect")
	}

	if len(jr.Requirements) != 1 || len(jr.Requirements["foo"]) != 1 || jr.Requirements["foo"][0] != "bar" {
		t.Fatal("JobRequest.Requirements are incorrect")
	}
}

func TestNewJobRequestIDGeneration(t *testing.T) {
	payloads := []JobPayload{JobPayload{"pong.service", "pong"}}
	jr, err := NewJobRequest(payloads, make([]string, 0), make(map[string][]string, 0))
	if err != nil {
		t.Errorf("Not expecting error:", err)
	}

	if len(jr.ID.String()) != 36 {
		t.Errorf("ID appears invalid: %s", jr.ID.String())
	}
}
